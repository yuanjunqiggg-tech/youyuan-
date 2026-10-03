$ErrorActionPreference='Stop'
$go='C:/Program Files/Go/bin/go.exe'
$ndk='D:/android-build/sdk/ndk/26.1.10909125/toolchains/llvm/prebuilt/windows-x86_64/bin'
$ProjectRoot=$PSScriptRoot
Set-Location (Join-Path $ProjectRoot 'third_party/cloudflared')
foreach($abi in @(@{Name='x86_64';Arch='amd64';Target='x86_64-linux-android26'},@{Name='arm64-v8a';Arch='arm64';Target='aarch64-linux-android26'})){
 $env:GOOS='android';$env:GOARCH=$abi.Arch;$env:CGO_ENABLED='1';$env:CC=(Join-Path $ndk 'clang.exe')+' --target='+$abi.Target
 & $go build -trimpath '-ldflags=-s -w -X main.Version=2026.9.3 -X main.BuildType=youyuan-android' -o (Join-Path $ProjectRoot ('upstream/android/app/src/main/jniLibs/'+$abi.Name+'/libyy_tunnel.so')) ./cmd/cloudflared
 if($LASTEXITCODE -ne 0){throw ('Tunnel compilation failed '+$abi.Name)}
 Write-Output ('Built cloudflared '+$abi.Name)
}
$env:GOOS='windows';$env:GOARCH='amd64';$env:CGO_ENABLED='0'
& $go build -trimpath '-ldflags=-s -w -X main.Version=2026.9.3' -o (Join-Path $ProjectRoot 'build/cloudflared.exe') ./cmd/cloudflared
if($LASTEXITCODE -ne 0){throw 'Windows tunnel compilation failed'}

$env:GOOS='linux';$env:GOARCH='amd64';$env:CGO_ENABLED='0'
& $go build -trimpath '-ldflags=-s -w -X main.Version=2026.9.3' -o (Join-Path $ProjectRoot 'build/cloudflared') ./cmd/cloudflared
if($LASTEXITCODE -ne 0){throw 'Linux tunnel compilation failed'}
