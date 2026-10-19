// Remove public source/developer archives, while preserving user plugin export
// and normal APK updates. Authoritative enforcement is at the public gateway.
export function isPrivateDistributionPath(path) {
 let decoded;try{decoded=decodeURIComponent(path)}catch{return true}
 if(decoded.includes('\\')||decoded.includes('\0'))return true;
 const segments=decoded.toLowerCase().split('/').filter(Boolean);
 // Authenticated project APIs must keep editing/exporting users' own code.
 if(['api','mcp'].includes(segments[0]))return false;
 return segments.some(x=>['.git','.env','node_modules','source.zip','plugin-sdk.zip','skill-library.zip','kernel.wasm','wasm_exec.js'].includes(x))
  || /\.(?:go|java|kt|ps1|sh|mjs|map|jks|keystore)$/i.test(decoded)
  || segments.some(x=>/^source(?:[-_.].*)?\.zip$/.test(x))
  || segments.some(x=>/(?:source|源码|源代码|sdk|开发资料|skill-library).*\.(?:zip|7z|tar|gz)$/i.test(x));
}
