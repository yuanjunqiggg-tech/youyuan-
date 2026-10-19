// Codes are bound to a normalized account name and mailbox, hashed at rest,
// single-use, and usable only after the execution node accepts the SMTP mail.
export const normalizeEmail = value => String(value || '').normalize('NFKC').trim().toLowerCase();
export const validQQEmail = value => /^[a-z0-9][a-z0-9._-]{0,63}@qq\.com$/.test(value);
export const validAccountName = value => /^[A-Za-z0-9_\-\u4e00-\u9fff]{3,32}$/.test(value);
const hex = bytes => [...new Uint8Array(bytes)].map(x => x.toString(16).padStart(2, '0')).join('');
export const emailDigest = async value => hex(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(value)));
export async function mailSignature(secret, timestamp, body) {
 const key = await crypto.subtle.importKey('raw', new TextEncoder().encode(secret), {name:'HMAC', hash:'SHA-256'}, false, ['sign']);
 return hex(await crypto.subtle.sign('HMAC', key, new TextEncoder().encode(timestamp + '\n' + body)));
}
export const verificationKey = async (name, email) => 'email-code:' + await emailDigest(name + '\n' + email);
export async function liveIndex(store, key) {
 const id = await store.get(key);
 if (!id) return null;
 if (await store.get('user:' + id)) return id;
 // Repair only a provably orphaned index; never replace an existing account.
 await store.delete(key); return null;
}
export async function requestRegistrationCode(request, input, ctx, env) {
 const name = String(input.username || '').normalize('NFKC').trim().toLowerCase(), email = normalizeEmail(input.email), now = Date.now();
 if (!validAccountName(name) || !validQQEmail(email)) return Response.json({ok:false, error:'请填写 3–32 字符账号名和有效 QQ 邮箱（例如 123456@qq.com）'}, {status:400});
 if (!env.CONSOLE_TOKEN || !env.RELAY) return Response.json({ok:false, error:'注册邮件服务尚未配置，请联系管理员'}, {status:503});
 const key = await verificationKey(name, email), ip = await emailDigest(request.headers.get('X-Identity-IP') || 'local');
 const bytes = new Uint32Array(1); let number;
 do { crypto.getRandomValues(bytes); number = bytes[0]; } while (number >= 4294000000);
 const code = String(number % 1000000).padStart(6, '0'), id = crypto.randomUUID(), salt = crypto.randomUUID();
 const record = {id, salt, hash:await emailDigest(salt + '\n' + code), name, email, sent:false, created:now, expires:now + 600000, attempts:0};
 const reserved = await ctx.storage.transaction(async store => {
  if (await liveIndex(store, 'name:' + name)) return {status:409, error:'账号已存在；退出登录不会注销账号，请登录或在平台账号中办理注销'};
  if (await liveIndex(store, 'email:' + email)) return {status:409, error:'该 QQ 邮箱已绑定账号，请登录原账号'};
  const emailRate = 'rate:mailbox:' + await emailDigest(email), mailbox = await store.get(emailRate);
  if (mailbox?.last && now - mailbox.last < 60000) return {status:429, error:'请等待 60 秒再发送验证码'};
  for (const [rateKey, limit] of [['rate:mail-ip:' + ip, 30], [emailRate, 8]]) {
   let rate = await store.get(rateKey) || {start:now, n:0};
   if (now - rate.start >= 86400000) rate = {start:now, n:0};
   if (rate.n >= limit) return {status:429, error:'今日验证码发送次数已用完，请稍后再试'};
  }
  for (const rateKey of ['rate:mail-ip:' + ip, emailRate]) {
   let rate = await store.get(rateKey) || {start:now, n:0};
   if (now - rate.start >= 86400000) rate = {start:now, n:0};
   rate.n++; rate.last = now; await store.put(rateKey, rate);
  }
  await store.put(key, record); return null;
 });
 if (reserved) return Response.json({ok:false, error:reserved.error}, {status:reserved.status});
 try {
  const timestamp = String(now), body = JSON.stringify({to:email, code, purpose:'registration'});
  const signature = await mailSignature(env.CONSOLE_TOKEN, timestamp, body);
  const relay = env.RELAY.get(env.RELAY.idFromName('linux-server'));
  const result = await relay.fetch(new Request('https://mail.internal/api/youyuan/mail/send', {method:'POST', headers:{'Content-Type':'application/json','X-YouYuan-Mail-Timestamp':timestamp,'X-YouYuan-Mail-Signature':signature}, body}));
  const answer = await result.json();
  if (!result.ok || answer.ok !== true) throw Error('delivery_failed');
  const activated = await ctx.storage.transaction(async store => { const current = await store.get(key); if (current?.id !== id) return false; current.sent = true; await store.put(key, current); return true; });
  if (!activated) return Response.json({ok:false,error:'验证码已被新的请求替换，请重新发送'}, {status:409});
  return Response.json({ok:true, message:'验证码邮件已提交，请查看 QQ 邮箱收件箱及垃圾箱', expires_in:600, retry_after:60});
 } catch {
  await ctx.storage.transaction(async store => {if ((await store.get(key))?.id === id) await store.delete(key);});
  return Response.json({ok:false, error:'验证码邮件未发出：发信服务尚未配置或暂不可用，请联系管理员；账号尚未创建'}, {status:503});
 }
}
// Called inside the same transaction that creates the account and consumes OTP.
export async function checkRegistrationCode(store, name, email, code, now) {
 const key = await verificationKey(name, email), record = await store.get(key);
 if (!record?.sent || record.expires <= now) return {status:400, error:'验证码未发送或已过期，请重新发送'};
 if (record.attempts >= 5) return {status:429, error:'验证码错误次数过多，请重新发送'};
 if (!/^\d{6}$/.test(code) || await emailDigest(record.salt + '\n' + code) !== record.hash) {
  record.attempts++; await store.put(key, record); return {status:400, error:'验证码不正确'};
 }
 return {key};
}
