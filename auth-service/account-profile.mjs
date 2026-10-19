// Only provider response data supplies the official account portrait.
export function officialPortrait(row){
 let client=row.client;try{if(typeof client==='string')client=JSON.parse(client)}catch{return ''}
 const detail=client?.UserDetail||client?.user_detail||{};
 for(const candidate of [detail.avatar_image_url,detail.head_image]){try{const u=new URL(candidate);if(u.protocol==='https:'&&!u.username&&!u.password)return u.href}catch{}}
 return '';
}
