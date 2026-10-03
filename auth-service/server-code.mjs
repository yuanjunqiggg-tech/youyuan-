// Realm invitation codes are different from numeric rental server numbers.
// Normalize user-facing invitation links before the direct NetEase kernel.
export function normalizeGameServer(value) {
  let server=String(value||'').trim();
  if(server.startsWith('mc.163.com/'))server='https://'+server;
  if(/^https?:\/\//i.test(server)) {
    try { const url=new URL(server);if(url.hostname==='mc.163.com')server=url.searchParams.get('realms')||server; } catch {}
  }
  if(server.startsWith('#'))return 'LobbyGame:'+server.slice(1);
  if(/^[0-9a-f]{11}$/i.test(server))return 'DomainGame:'+server;
  return server;
}
