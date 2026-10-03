$ErrorActionPreference='Stop'
Push-Location $PSScriptRoot
try{
 $Go='C:/Program Files/Go/bin/go.exe'
 if(!(Test-Path -LiteralPath $Go)){$Go='go'}
 $runtimeRoot=& $Go env GOROOT
 Copy-Item -LiteralPath (Join-Path $runtimeRoot 'lib/wasm/wasm_exec.js') -Destination wasm_exec.js
 Copy-Item -LiteralPath (Join-Path $runtimeRoot 'LICENSE') -Destination GO-RUNTIME-LICENSE
 $env:GOOS='js';$env:GOARCH='wasm';$env:CGO_ENABLED='0'
 & $Go build -mod=mod -trimpath -ldflags='-s -w' -o kernel.wasm .
 if($LASTEXITCODE -ne 0){throw '认证WASM构建失败'}
}finally{Remove-Item Env:GOOS,Env:GOARCH,Env:CGO_ENABLED -ErrorAction SilentlyContinue;Pop-Location}
