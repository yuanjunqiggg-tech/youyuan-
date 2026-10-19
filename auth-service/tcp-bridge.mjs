// Request-owned bridge. There is no public TCP proxy route and no account data
// in this adapter. Destinations come only from the SDK's official discovery.
export function createTCPBridge(connect) {
 const sockets=new Set();
 const dial=address=>{
  const match=/^([0-9.]+):([0-9]{1,5})$/.exec(address);
  if(!match||!match[1].split('.').every(n=>/^\d{1,3}$/.test(n)&&Number(n)<=255)||match[1].split('.').length!==4)throw Error('Invalid Link endpoint');
  const host=match[1],port=Number(match[2]),first=Number(host.split('.')[0]),second=Number(host.split('.')[1]);
  if(![10200,10201].includes(port)||first===0||first===10||first===127||first>=224||(first===169&&second===254)||(first===172&&second>=16&&second<=31)||(first===192&&second===168))throw Error('Unsupported Link endpoint');
  const socket=connect({hostname:host,port},{secureTransport:'off',allowHalfOpen:false});
  sockets.add(socket);socket.closed.catch(()=>{});
  let reader,writer,pending=new Uint8Array(),closed=false;
  const close=()=>{if(closed)return;closed=true;sockets.delete(socket);reader?.cancel().catch(()=>{});writer?.abort().catch(()=>{});socket.close().catch(()=>{});};
  return {
   opened:socket.opened.catch(error=>{close();throw error;}),
   async read(size){
    if(closed)throw Error('Link socket closed');
    if(!Number.isInteger(size)||size<1||size>1048576)throw Error('Invalid read size');
    reader??=socket.readable.getReader();
    while(!pending.length){const result=await reader.read();if(result.done)return new Uint8Array();pending=result.value;}
    const value=pending.slice(0,size);pending=pending.subarray(value.length);return value;
   },
   async write(bytes){if(closed)throw Error('Link socket closed');writer??=socket.writable.getWriter();await writer.write(bytes);return bytes.length;},
   close
  };
 };
 return {dial,close(){for(const socket of sockets){socket.close().catch(()=>{});}sockets.clear();}};
}
