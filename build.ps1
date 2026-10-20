param([string]$JavaHome='D:/android-build/jdk',[string]$SdkRoot='D:/android-build/sdk',[string]$Gradle='D:/android-build/gradle/bin/gradle.bat',[string]$Python='D:/YouYuan-Codex/.venv/Scripts/python.exe',[string]$SigningConfigPath='',[string]$SigningKeystorePath='')
$ErrorActionPreference='Stop'
$ProjectRoot=$PSScriptRoot
New-Item -ItemType Directory -Force (Join-Path $ProjectRoot 'build'),(Join-Path $ProjectRoot 'private') | Out-Null
$Go='C:/Program Files/Go/bin/go.exe'
$NdkBin=Join-Path $SdkRoot 'ndk/26.1.10909125/toolchains/llvm/prebuilt/windows-x86_64/bin'
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
 & $Go build -buildmode=c-shared -trimpath '-ldflags=-s -w -checklinkname=0' -o (Join-Path $NativeDir 'libyouyuan.so') .
 if($LASTEXITCODE -ne 0){throw ('Android Go 编译失败 '+$abi.Name)}
}
$env:CGO_ENABLED='0';$env:GOARCH='amd64';$env:GOOS='windows'
& $Go build -trimpath '-ldflags=-s -w' -o (Join-Path $ProjectRoot 'build/youyuan-windows.exe') .
if($LASTEXITCODE -ne 0){throw 'Windows 编译失败'}
$env:GOOS='linux'
& $Go build -trimpath '-ldflags=-s -w' -o (Join-Path $ProjectRoot 'build/youyuan-linux-x86_64') .
if($LASTEXITCODE -ne 0){throw 'Linux 编译失败'}
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
