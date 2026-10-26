// Cloudflare Turnstile 校验（平台账号注册 / 登录前置真人确认）。
//
// 与 auth-service/turnstile.mjs 同一套语义：
// - 未配置 TURNSTILE_SECRET_KEY 时直接放行（软启动，避免部署即锁死登录）
// - 一旦配置 secret，注册与登录必须带 turnstile_token
// - 站点密钥是公开信息，通过 /api/identity/turnstile-config 下发给前端
const VERIFY_URL = 'https://challenges.cloudflare.com/turnstile/v0/siteverify';

export function turnstileEnabled(env) {
  return !!env.TURNSTILE_SECRET_KEY;
}

export function turnstileSiteKey(env) {
  return env.TURNSTILE_SITE_KEY || '';
}

export async function verifyTurnstile(env, input, remoteip) {
  const secret = env.TURNSTILE_SECRET_KEY;
  if (!secret) return null;
  const token = String(input.turnstile_token || input['cf-turnstile-response'] || '').trim();
  if (!token) throw Error('请先完成人机验证（勾选“我是人类”或按提示点击）');
  if (token.length > 4096) throw Error('人机验证凭据格式无效，请重新验证');
  const body = new URLSearchParams({secret, response: token});
  if (remoteip) body.set('remoteip', remoteip);
  let payload;
  try {
    const r = await fetch(VERIFY_URL, {method: 'POST', body});
    payload = await r.json();
  } catch {
    throw Error('人机验证服务暂时不可达，请稍后重试');
  }
  if (!payload || !payload.success) {
    const codes = ((payload && payload['error-codes']) || []).join(',');
    throw Error('人机验证未通过，请重新验证后再提交' + (codes ? `（${codes}）` : ''));
  }
  return payload;
}

export function turnstileRemoteip(request) {
  return request.headers.get('CF-Connecting-IP') || '';
}

// ---------------------------------------------------------------------------
// 本地工作量证明（PoW）兜底
//
// 现实问题：challenges.cloudflare.com 在国内部分网络（尤其移动蜂窝网络）
// 加载很慢甚至完全不可达，一旦加载不出来，注册与登录就彻底死锁。
// 所以保留 Turnstile 作为首选，同时提供一个不依赖第三方的兜底：
// 服务端签发带签名的挑战，前端算一个 SHA-256 前 N 位为 0 的 nonce。
// 强度不如 Turnstile，但足以挡住无脑脚本，且永远不会因为网络问题锁死用户。
// ---------------------------------------------------------------------------
const POW_DIFFICULTY = 4;
const POW_TTL_MS = 5 * 60 * 1000;

function powSecret(env) {
  return env.TURNSTILE_SECRET_KEY || 'yy-pow-fallback';
}

async function hmacHex(secret, msg) {
  const key = await crypto.subtle.importKey('raw', new TextEncoder().encode(secret), {name: 'HMAC', hash: 'SHA-256'}, false, ['sign']);
  const sig = await crypto.subtle.sign('HMAC', key, new TextEncoder().encode(msg));
  return [...new Uint8Array(sig)].map(b => b.toString(16).padStart(2, '0')).join('');
}

export async function issuePowChallenge(env) {
  const salt = [...crypto.getRandomValues(new Uint8Array(12))].map(b => b.toString(16).padStart(2, '0')).join('');
  const expires = Date.now() + POW_TTL_MS;
  const sig = await hmacHex(powSecret(env), `${salt}.${POW_DIFFICULTY}.${expires}`);
  return {salt, difficulty: POW_DIFFICULTY, expires, sig, algo: 'sha256-prefix-zero'};
}

export async function verifyPowProof(env, proof) {
  if (!proof || typeof proof !== 'object') throw Error('缺少人机验证凭据');
  const salt = String(proof.salt || ''), nonce = String(proof.nonce || '');
  const difficulty = Number(proof.difficulty || 0), expires = Number(proof.expires || 0);
  if (!salt || !nonce || !difficulty || !expires) throw Error('人机验证凭据不完整');
  if (difficulty !== POW_DIFFICULTY) throw Error('人机验证难度不匹配');
  if (Date.now() > expires) throw Error('人机验证已过期，请重新验证');
  if (salt.length > 64 || nonce.length > 64) throw Error('人机验证凭据格式无效');
  const expect = await hmacHex(powSecret(env), `${salt}.${difficulty}.${expires}`);
  if (expect !== String(proof.sig || '')) throw Error('人机验证凭据无效');
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(`${salt}.${nonce}`));
  const hex = [...new Uint8Array(digest)].map(b => b.toString(16).padStart(2, '0')).join('');
  if (!hex.startsWith('0'.repeat(difficulty))) throw Error('人机验证未通过，请重新验证');
  return true;
}

// requireHuman 统一入口：优先 Turnstile，缺失/失败时接受 PoW 兜底。
export async function requireHuman(env, input, remoteip) {
  try {
    await verifyTurnstile(env, input, remoteip);
    return 'turnstile';
  } catch (turnstileError) {
    if (input && input.pow_proof) {
      await verifyPowProof(env, input.pow_proof);
      return 'pow';
    }
    throw turnstileError;
  }
}
