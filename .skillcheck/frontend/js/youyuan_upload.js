(function() {
  const CHUNK = 1024 * 1024;
  const hash = async (blob) => Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", await blob.arrayBuffer())), (n) => n.toString(16).padStart(2, "0")).join("");
  function ticketId() {
    return Array.from(crypto.getRandomValues(new Uint8Array(16)), (n) => n.toString(16).padStart(2, "0")).join("");
  }
  function readStore(key) {
    try {
      return JSON.parse(localStorage.getItem(key) || "null");
    } catch {
      return null;
    }
  }
  function saveStore(key, value) {
    try {
      if (value) localStorage.setItem(key, JSON.stringify(value));
      else localStorage.removeItem(key);
    } catch {
    }
  }
  function pause(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }
  function invalid(message) {
    const error = Error(message);
    error.status = 409;
    error.nonRetryable = true;
    return error;
  }
  function platformID() {
    return typeof yyPlatformUser !== "undefined" && yyPlatformUser && typeof yyPlatformUser.id === "string" ? yyPlatformUser.id : "";
  }
  function cloudKey() {
    try {
      return sessionStorage.getItem("yy-cloud-key") || "local";
    } catch {
      return "local";
    }
  }
  window.yyUploadFile = async function(file, options) {
    options = options || {};
    if (!file || !Number.isSafeInteger(file.size) || file.size < 0 || file.size > 2 * 1024 * 1024 * 1024) throw Error("\u8BF7\u9009\u62E9\u4E0D\u8D85\u8FC72GiB\u7684\u6587\u4EF6");
    if (!crypto.subtle) throw Error("\u5F53\u524D\u9875\u9762\u7F3A\u5C11\u5B89\u5168\u6821\u9A8C\u80FD\u529B\uFF0C\u8BF7\u4F7F\u7528HTTPS\u6216\u5E94\u7528\u5185\u9875\u9762");
    const account = typeof yyActiveAccount === "string" ? yyActiveAccount : "main", owner = platformID();
    const base = typeof yyAccountPath === "function" ? yyAccountPath("/api/youyuan/files/transfer", account) : "/api/youyuan/files/transfer";
    const device = new URLSearchParams(location.search).get("device") || "local", credential = cloudKey();
    const identity = JSON.stringify([location.origin, device, account, owner ? "user:" + owner : "key:" + credential]), scope = await hash(new Blob([identity]));
    const fingerprint = await hash(new Blob([file.name, "|", String(file.size), "|", String(file.lastModified), "|", await hash(file.slice(0, CHUNK)), "|", await hash(file.slice(Math.max(0, file.size - CHUNK)))]));
    const key = "yy-upload:" + scope.slice(0, 16) + ":" + fingerprint;
    let saved = readStore(key), upload, verifiedOffset = 0;
    if (!saved) {
      const legacyScope = await hash(new Blob([[location.origin, device, account, credential].join("|")]));
      saved = readStore("yy-upload:" + legacyScope.slice(0, 16) + ":" + fingerprint);
      if (saved) saveStore(key, saved);
    }
    if (saved && !/^[a-f0-9]{32}$/.test(saved.id || "")) {
      saveStore(key, null);
      saved = null;
    }
    const progress = (message) => {
      if (options.onProgress) options.onProgress({ name: file.name, total: file.size, offset: (upload == null ? void 0 : upload.offset) || 0, message });
    };
    function active() {
      var _a;
      if ((_a = options.signal) == null ? void 0 : _a.aborted) {
        const e = Error("\u5DF2\u6682\u505C\u4E0A\u4F20\uFF0C\u91CD\u65B0\u9009\u62E9\u540C\u4E00\u6587\u4EF6\u53EF\u4ECE\u5DF2\u786E\u8BA4\u4F4D\u7F6E\u7EE7\u7EED");
        e.status = 499;
        e.nonRetryable = true;
        throw e;
      }
      if ((typeof yyActiveAccount === "string" ? yyActiveAccount : "main") !== account || (owner ? platformID() !== owner : cloudKey() !== credential)) {
        const e = Error("\u64CD\u4F5C\u8D26\u53F7\u5DF2\u5207\u6362\uFF0C\u4E0A\u4F20\u5DF2\u6682\u505C\uFF1B\u8BF7\u5207\u56DE\u539F\u8D26\u53F7\u540E\u7EE7\u7EED");
        e.status = 499;
        e.nonRetryable = true;
        throw e;
      }
    }
    function validateTicket(t) {
      if (!t || t.id !== saved.id || t.name !== file.name || t.size !== file.size || !Number.isSafeInteger(t.offset) || t.offset < 0 || t.offset > file.size || typeof t.complete !== "boolean" || !Array.isArray(t.chunks)) throw invalid("\u7EED\u4F20\u8BB0\u5F55\u4E0E\u5F53\u524D\u6587\u4EF6\u4E0D\u4E00\u81F4\uFF0C\u672A\u786E\u8BA4\u4E0A\u4F20\u6210\u529F");
      return t;
    }
    function completed(t, resumed) {
      active();
      validateTicket(t);
      if (!t.complete || t.offset !== file.size || typeof t.path !== "string" || !t.path.trim() || t.path.includes("\0") || !t.path.split(/[\\/]/).pop() || !/^[a-f0-9]{64}$/.test(t.sha256 || "")) throw invalid("\u4E0A\u4F20\u7ED3\u679C\u7F3A\u5C11\u6709\u6548\u8DEF\u5F84\u3001\u5B57\u8282\u6570\u6216SHA-256\uFF0C\u672A\u786E\u8BA4\u6587\u4EF6\u4FDD\u5B58\u6210\u529F");
      progress("\u9010\u5757\u5185\u5BB9\u6821\u9A8C\u901A\u8FC7\uFF0C\u6587\u4EF6\u5DF2\u4FDD\u5B58");
      return { ok: true, path: t.path, name: t.path.split(/[\\/]/).pop(), bytes: t.size, sha256: t.sha256, ...resumed ? { resumed: true } : {} };
    }
    async function request(method, suffix, body) {
      var _a, _b;
      active();
      const control = new AbortController(), timer = setTimeout(() => control.abort(), 45e3), abort = () => control.abort();
      (_a = options.signal) == null ? void 0 : _a.addEventListener("abort", abort, { once: true });
      try {
        const response = await fetch(base + (suffix || ""), { method, headers: { "Content-Type": body instanceof Blob ? "application/octet-stream" : "application/json" }, body: body === void 0 ? void 0 : body instanceof Blob ? body : JSON.stringify(body), signal: control.signal });
        const result = await response.json().catch(() => ({ ok: false, error: "\u4E0A\u4F20\u63A5\u53E3\u672A\u8FD4\u56DE\u6709\u6548\u7ED3\u679C" }));
        active();
        if (!response.ok || !result.ok) {
          const e = Error(result.error || "\u4E0A\u4F20\u5931\u8D25");
          e.status = response.status;
          throw e;
        }
        return result;
      } catch (error) {
        active();
        throw error;
      } finally {
        clearTimeout(timer);
        (_b = options.signal) == null ? void 0 : _b.removeEventListener("abort", abort);
      }
    }
    async function state() {
      return validateTicket((await request("GET", "?id=" + saved.id)).upload);
    }
    async function verifyPrefix(t) {
      validateTicket(t);
      let expected = 0;
      for (const c of t.chunks) {
        active();
        if (!Number.isSafeInteger(c.offset) || c.offset !== expected || !Number.isSafeInteger(c.size) || c.size <= 0 || c.size > CHUNK || c.offset + c.size > file.size || !/^[a-f0-9]{64}$/.test(c.sha256 || "") || await hash(file.slice(c.offset, c.offset + c.size)) !== c.sha256) throw invalid("\u672C\u5730\u6587\u4EF6\u4E0E\u5DF2\u786E\u8BA4\u7684\u4E0A\u4F20\u524D\u7F00\u4E0D\u540C\uFF0C\u8BF7\u91CD\u65B0\u9009\u62E9\u539F\u6587\u4EF6");
        expected += c.size;
      }
      if (expected !== t.offset) throw invalid("\u7EED\u4F20\u8BB0\u5F55\u7684\u5DF2\u786E\u8BA4\u5206\u5757\u4E0E\u5B57\u8282\u4F4D\u7F6E\u4E0D\u4E00\u81F4");
      verifiedOffset = t.offset;
    }
    active();
    if (saved) {
      try {
        upload = await state();
        progress("\u6B63\u5728\u9010\u5757\u6838\u5BF9\u5DF2\u786E\u8BA4\u7684\u4E0A\u4F20\u524D\u7F00\u2026");
        await verifyPrefix(upload);
      } catch (e) {
        if (e.status === 404) {
          saveStore(key, null);
          saved = null;
        } else throw e;
      }
    }
    if (!saved) {
      saved = { id: ticketId() };
      saveStore(key, saved);
      progress("\u6B63\u5728\u5EFA\u7ACB\u4E0A\u4F20\u2026");
      try {
        upload = validateTicket((await request("POST", "", { id: saved.id, name: file.name, size: file.size })).upload);
      } catch (e) {
        if (e.nonRetryable) throw e;
        try {
          upload = await state();
        } catch (check) {
          if (check.nonRetryable) throw check;
          throw Error("\u6682\u672A\u786E\u8BA4\u4E0A\u4F20\u8FDE\u63A5\uFF1B\u91CD\u65B0\u9009\u62E9\u540C\u4E00\u6587\u4EF6\u53EF\u7EE7\u7EED\uFF0C\u4E0D\u4F1A\u91CD\u590D\u5BFC\u5165\u3002" + e.message);
        }
      }
      if (upload.offset) await verifyPrefix(upload);
    }
    if (upload.complete) return completed(upload, true);
    let failures = 0;
    while (upload.offset < file.size) {
      active();
      const offset = upload.offset, chunk = file.slice(offset, offset + CHUNK), digest = await hash(chunk);
      progress("\u6B63\u5728\u9010\u5757\u4E0A\u4F20 " + Math.floor(offset / Math.max(1, file.size) * 100) + "%");
      try {
        const next = validateTicket((await request("PUT", "?id=" + saved.id + "&offset=" + offset + "&sha256=" + digest, chunk)).upload);
        if (next.offset < offset + chunk.size) throw invalid("\u670D\u52A1\u5668\u672A\u786E\u8BA4\u5F53\u524D\u5206\u5757\uFF0C\u4E0A\u4F20\u5C1A\u672A\u5B8C\u6210");
        if (next.offset !== offset + chunk.size) await verifyPrefix(next);
        upload = next;
        verifiedOffset = upload.offset;
        failures = 0;
      } catch (e) {
        if (e.nonRetryable || e.status && e.status < 500 && e.status !== 429) throw e;
        if (++failures > 4) throw Error("\u4E0A\u4F20\u5DF2\u6682\u505C\uFF1B\u91CD\u65B0\u9009\u62E9\u540C\u4E00\u6587\u4EF6\u5373\u53EF\u4ECE\u5DF2\u786E\u8BA4\u4F4D\u7F6E\u7EE7\u7EED\u3002" + e.message);
        progress("\u8FDE\u63A5\u4E2D\u65AD\uFF0C\u6B63\u5728\u6838\u5BF9\u5DF2\u63A5\u6536\u4F4D\u7F6E\u2026");
        await pause(400 * failures);
        try {
          const latest = await state();
          if (latest.offset !== verifiedOffset) await verifyPrefix(latest);
          upload = latest;
        } catch (check) {
          if (check.nonRetryable || check.status && check.status < 500 && check.status !== 429) throw check;
        }
      }
    }
    progress("\u6B63\u5728\u7531\u6267\u884C\u8BBE\u5907\u590D\u6838\u5DF2\u63A5\u6536\u5206\u5757\u5E76\u8BA1\u7B97\u6587\u4EF6SHA-256\u2026");
    try {
      upload = validateTicket((await request("POST", "?id=" + saved.id)).upload);
    } catch (e) {
      if (e.nonRetryable) throw e;
      const latest = await state();
      if (!latest.complete) throw e;
      upload = latest;
    }
    return completed(upload, false);
  };
})();
