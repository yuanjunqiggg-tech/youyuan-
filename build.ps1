param([string]$JavaHome='D:/android-build/jdk',[string]$SdkRoot='D:/android-build/sdk',[string]$Gradle='D:/android-build/gradle/bin/gradle.bat',[string]$Python='D:/YouYuan-Codex/.venv/Scripts/python.exe',[string]$SigningConfigPath='',[string]$SigningKeystorePath='',[switch]$Pie)
$ErrorActionPreference='Stop'
$ProjectRoot=$PSScriptRoot
New-Item -ItemType Directory -Force (Join-Path $ProjectRoot 'build'),(Join-Path $ProjectRoot 'private') | Out-Null
$Go='C:/Program Files/Go/bin/go.exe'
$NdkBin=Join-Path $SdkRoot 'ndk/26.1.10909125/toolchains/llvm/prebuilt/windows-x86_64/bin'
# 构建指纹：写进二进制，方便事后把线上崩溃/安全日志对回某一次构建。
# -buildid= 清掉 Go 自带的 build id，让两次构建的产物内部指纹无法互相定位。
$Stamp=(Get-Date -Format 'yyyyMMdd-HHmmss')+'-'+(-join ((48..57)+(97..122) | Get-Random -Count 6 | ForEach-Object {[char]$_}))
# 内嵌前端资源树摘要。upstream/frontend 整个目录会经 //go:embed 编进二进制，
# 改一个 JS 再重签 APK 就会让这里的摘要对不上。
# 算法必须与 upstream/integrity.go 的 frontendResourceDigest 逐字节一致：
#   递归所有文件 → 按 "frontend/<相对路径>" 字节序排序 → 每个文件拼 "路径\n<sha256 小写>\n"
#   → 对整段 UTF-8 文本再求 sha256。
# 少了这行注入，RunIntegrityChecks 会直接判失败（见 integrity.go），
# 所以不存在「构建时忘了传就静默通过」的口子。
function Get-YYFrontendDigest([string]$Root){
  if(!(Test-Path -LiteralPath $Root)){throw "前端资源目录不存在：$Root"}
  $files=@(Get-ChildItem -LiteralPath $Root -Recurse -File -Force)
  if($files.Count -eq 0){throw '前端资源目录为空，拒绝构建'}
  $rel=@{}
  foreach($f in $files){
    $p=$f.FullName.Substring($Root.Length).TrimStart('\','/').Replace('\','/')
    $rel['frontend/'+$p]=$f.FullName
  }
  $keys=@($rel.Keys)
  [Array]::Sort($keys,[System.StringComparer]::Ordinal)
  $sb=New-Object System.Text.StringBuilder
  foreach($k in $keys){
    $h=(Get-FileHash -LiteralPath $rel[$k] -Algorithm SHA256).Hash.ToLowerInvariant()
    [void]$sb.Append($k).Append("`n").Append($h).Append("`n")
  }
  $sha=[System.Security.Cryptography.SHA256]::Create()
  try{
    $bytes=[System.Text.Encoding]::UTF8.GetBytes($sb.ToString())
    return ([BitConverter]::ToString($sha.ComputeHash($bytes)) -replace '-','').ToLowerInvariant()
  } finally { $sha.Dispose() }
}
$FrontendHash=Get-YYFrontendDigest (Join-Path $PSScriptRoot 'upstream/frontend')
Write-Output ('前端资源摘要 sha256:'+$FrontendHash)
# 通用加固参数：剥离符号与调试信息(-s -w) + 去 buildid + 注入构建指纹。
$HardLDFlags='-s -w -buildid= -X main.buildStamp='+$Stamp+' -X main.frontendHash=sha256:'+$FrontendHash
Write-Output ('构建指纹 '+$Stamp)
$env:JAVA_HOME=$JavaHome
$env:ANDROID_HOME=$SdkRoot
$env:YY_BUILD_PYTHON=$Python
$TunnelFiles=@('upstream/android/app/src/main/jniLibs/x86_64/libyy_tunnel.so','upstream/android/app/src/main/jniLibs/arm64-v8a/libyy_tunnel.so','build/cloudflared.exe')
if(@($TunnelFiles | Where-Object {!(Test-Path -LiteralPath (Join-Path $ProjectRoot $_))}).Count -gt 0){& (Join-Path $ProjectRoot 'build-tunnel.ps1');if($LASTEXITCODE -ne 0){throw '内置隧道构建失败'}}
Set-Location (Join-Path $ProjectRoot 'upstream')
& $Python (Join-Path $ProjectRoot 'generate_catalog.py')
foreach($abi in @(@{Name='x86_64';Arch='amd64';Target='x86_64-linux-android26'},@{Name='arm64-v8a';Arch='arm64';Target='aarch64-linux-android26'})) {
 $NativeDir=Join-Path (Get-Location) ('android/app/src/main/jniLibs/'+$abi.Name)
 New-Item -ItemType Directory -Force $NativeDir | Out-Null
 & (Join-Path $NdkBin 'clang++.exe') ('--target='+$abi.Target) -shared -fPIC -O3 -static-libstdc++ -o (Join-Path $NativeDir 'libyy_counter.so') (Join-Path $ProjectRoot 'native/counter.cpp')
 if($LASTEXITCODE -ne 0){throw 'C++ 编译失败'}
 & (Join-Path $NdkBin 'clang++.exe') ('--target='+$abi.Target) -std=c++17 -shared -fPIC -O2 -static-libstdc++ -I (Join-Path $ProjectRoot 'upstream/sdk/cpp') -o (Join-Path $NativeDir 'libyy_sdk_demo.so') (Join-Path $ProjectRoot 'samples/sdk_cpp_control/main.cpp')
 if($LASTEXITCODE -ne 0){throw 'C++ SDK 控制插件编译失败'}
 & (Join-Path $NdkBin 'clang++.exe') ('--target='+$abi.Target) -std=c++17 -shared -fPIC -O2 -static-libstdc++ -I (Join-Path $ProjectRoot 'upstream/sdk/cpp') -o (Join-Path $NativeDir 'libyy_observer.so') (Join-Path $ProjectRoot 'upstream/sdk/cpp_observer.cpp')
 if($LASTEXITCODE -ne 0){throw 'C++ 感知插件编译失败'}
 $env:GOOS='android';$env:GOARCH=$abi.Arch;$env:CGO_ENABLED='1';$env:CC=(Join-Path $NdkBin 'clang.exe')+' --target='+$abi.Target
 & $Go build -buildmode=c-shared -trimpath ('-ldflags=-s -w -checklinkname=0 -buildid= -X main.buildStamp='+$Stamp+' -X main.frontendHash=sha256:'+$FrontendHash) -o (Join-Path $NativeDir 'libyouyuan.so') .
 if($LASTEXITCODE -ne 0){throw ('Android Go 编译失败 '+$abi.Name)}
}
$env:CGO_ENABLED='0';$env:GOARCH='amd64';$env:GOOS='windows'
& $Go build -trimpath ('-ldflags='+$HardLDFlags) -o (Join-Path $ProjectRoot 'build/youyuan-windows.exe') .
if($LASTEXITCODE -ne 0){throw 'Windows 编译失败'}
# 封印：把「槽位清零后的自身 sha256」回填进二进制（见 upstream/protect_selfcheck.go）。
# 必须在 build 之后、任何 strip/upx 之前完成；封印后文件不可再被改写。
& $Python (Join-Path $ProjectRoot 'maintenance/seal_binary.py') (Join-Path $ProjectRoot 'build/youyuan-windows.exe')
if($LASTEXITCODE -ne 0){throw 'Windows 二进制封印失败'}

$env:GOOS='linux'
# -Pie：把 Linux 二进制改成 PIE（ET_DYN），让主程序也吃到 ASLR。
# 默认不开：现在的产物是 CGO_ENABLED=0 的全静态 ET_EXEC，丢到任何发行版都能跑；
# 改成 PIE 会引入对动态链接器（/lib64/ld-linux-x86-64.so.2）的硬依赖，
# 极小镜像 / musl 环境上会直接起不来。确认目标机有 glibc 再用。
$PieArgs=@(); if($Pie){$PieArgs=@('-buildmode=pie')}
& $Go build @PieArgs -trimpath ('-ldflags='+$HardLDFlags) -o (Join-Path $ProjectRoot 'build/youyuan-linux-x86_64') .
if($LASTEXITCODE -ne 0){throw 'Linux 编译失败'}
& $Python (Join-Path $ProjectRoot 'maintenance/seal_binary.py') (Join-Path $ProjectRoot 'build/youyuan-linux-x86_64')
if($LASTEXITCODE -ne 0){throw 'Linux 二进制封印失败'}

Remove-Item Env:GOOS,Env:GOARCH,Env:CGO_ENABLED,Env:CC -ErrorAction SilentlyContinue
Set-Location android
& $Gradle --offline --no-daemon --console=plain assembleRelease
if($LASTEXITCODE -ne 0){throw 'APK 打包失败'}
$SigningPath=if($SigningConfigPath){$SigningConfigPath}else{'D:/YouYuan-APK/private/signing.json'}
if(!(Test-Path -LiteralPath $SigningPath)){throw "签名密码文件缺失：$SigningPath（绝不能自动生成新密钥，否则与线上证书不一致导致无法覆盖安装）"}
$env:YY_SIGN_PASSWORD=(Get-Content -Raw $SigningPath | ConvertFrom-Json).password
$Keystore=if($SigningKeystorePath){$SigningKeystorePath}else{'D:/YouYuan-APK/private/youyuan-release.jks'}
if(!(Test-Path -LiteralPath $Keystore)) {
 throw "签名密钥缺失：$Keystore（证书与线上发布的 SHA-256 cd823622… 绑定，不可重新生成）"
}
$BuildTools=Join-Path $SdkRoot 'build-tools/34.0.0'
$Aligned=Join-Path $ProjectRoot 'build/youyuan-aligned.apk'
& (Join-Path $BuildTools 'zipalign.exe') -f -p 4 app/build/outputs/apk/release/app-release-unsigned.apk $Aligned
if($LASTEXITCODE -ne 0){throw 'APK 对齐失败'}
& (Join-Path $BuildTools 'apksigner.bat') sign --ks $Keystore --ks-key-alias youyuan --ks-pass env:YY_SIGN_PASSWORD --key-pass env:YY_SIGN_PASSWORD --out (Join-Path $ProjectRoot 'build/youyuan-v0.10.1.apk') $Aligned
if($LASTEXITCODE -ne 0){throw 'APK 签名失败'}
& (Join-Path $BuildTools 'apksigner.bat') verify --verbose --print-certs (Join-Path $ProjectRoot 'build/youyuan-v0.10.1.apk')
if($LASTEXITCODE -ne 0){throw 'APK 签名验证失败'}
Remove-Item Env:YY_SIGN_PASSWORD
Write-Output '三个平台的构建产物已生成，APK 使用游园自己的签名。'
