import './wasm_exec.js';
import wasm from './kernel.wasm';
import {connect} from 'cloudflare:sockets';
import {createTCPBridge} from './tcp-bridge.mjs';
export async function runKernel(input){
 // Each invocation owns its Cloudflare request I/O context. A retained Go
 // instance would keep the first request's timers and fetch continuation.
 const go=new globalThis.Go();const instance=await WebAssembly.instantiate(wasm,go.importObject);
 go.exit=()=>{for(const id of go._scheduledTimeouts.values())clearTimeout(id);go._scheduledTimeouts.clear()};
 const running=go.run(instance);
 const invoke=globalThis.youyuanAuthKernel;
 if(typeof invoke!=='function')throw new Error('认证运行时未就绪');
 const tcp=createTCPBridge(connect);
 try {
  const result=await invoke(JSON.stringify(input),tcp.dial);
  await running;
  return JSON.parse(result);
 } finally {tcp.close();}
}
