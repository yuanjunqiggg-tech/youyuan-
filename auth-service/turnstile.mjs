// Cloudflare Turnstile 校验。
// 设计原则：TURNSTILE_SECRET_KEY 未配置时直接放行（软启动，部署即生效但不锁注册）；
// 一旦在 wrangler secret 里配置了 secret，所有注册/批量入口必须通过人机验证。
// 前端对应：public/index.html 加载 challenges.cloudflare.com/turnstile/v0/api.js，
// 注册表单渲染 widget，提交时带 turnstile_token 字段。
const VERIFY_URL = 'https://challenges.cloudflare.com/turnstile/v0/siteverify';

export async function verifyTurnstile(env, input, remoteip) {
  const secret = env.TURNSTILE_SECRET_KEY;
  if (!secret) return null; // 未启用，放行
  const token = String(input.turnstile_token || input['cf-turnstile-response'] || '').trim();
  if (!token) throw Error('请先完成人机验证（勾选“我是人类”或按提示点击）');
  const body = new URLSearchParams({secret, response: token});
  if (remoteip) body.set('remoteip', remoteip);
  let payload;
  try {
    const r = await fetch(VERIFY_URL, {method: 'POST', body});
    payload = await r.json();
  } catch {
    throw Error('人机验证服务暂时不可达，请稍后重试');
  }
  if (!payload.success) {
    const codes = (payload['error-codes'] || []).join(',');
    throw Error('人机验证未通过，请重新点击验证后提交' + (codes ? `（${codes}）` : ''));
  }
  return payload;
}

export function turnstileRemoteip(request) {
  return request.headers.get('CF-Connecting-IP') || '';
}
