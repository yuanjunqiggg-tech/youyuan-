# -*- coding: utf-8 -*-
"""Extract user messages from Codex CLI session files for the APK project."""
import os, sys, json, io, re

try:
    import zstandard
    HAS_ZSTD = True
except ImportError:
    HAS_ZSTD = False

CODEX_DIR = r"C:\Users\Lenovo\.codex"
PROJECT = "2026-09-28"
OUT = r"C:\Users\Lenovo\Documents\Codex\2026-09-28\apk\project\outputs\_user_messages_raw.jsonl"

def open_text(path):
    """Return text stream, decompressing .zst if needed."""
    if path.endswith(".zst"):
        if not HAS_ZSTD:
            return None
        dctx = zstandard.ZstdDecompressor()
        fh = open(path, "rb")
        try:
            return io.TextIOWrapper(dctx.stream_reader(fh), encoding="utf-8", errors="replace")
        except Exception:
            fh.close()
            return None
    return open(path, "r", encoding="utf-8", errors="replace")

files = []
for root, dirs, fnames in os.walk(CODEX_DIR):
    dirs[:] = [d for d in dirs if d not in (".sandbox", ".sandbox-bin", ".sandbox-secrets", "vendor_imports", "node_repl", "cache", "browser", "plugins", "skills", "tmp", ".tmp", "pets", "visualizations", "attachments")]
    for fn in fnames:
        if fn.startswith("rollout-") and (fn.endswith(".jsonl") or fn.endswith(".jsonl.zst")):
            files.append(os.path.join(root, fn))

print(f"session files found: {len(files)}, zstd available: {HAS_ZSTD}")

SKIP_PREFIXES = ("<environment_context", "<user_instructions", "<turn_context", "# AGENTS", "<system", "[EXPIRED", "<permissions", "<app_context", "<ide_context")
seen = set()
results = []

def record(ts, fpath, role_tag, text):
    t = text.strip()
    if not t or len(t) < 2:
        return
    low = t[:200].lower()
    if any(t.lstrip().startswith(p) for p in SKIP_PREFIXES):
        return
    key = (fpath, t[:300])
    if key in seen:
        return
    seen.add(key)
    results.append({"ts": ts, "file": fpath, "tag": role_tag, "text": t})

for fpath in sorted(files):
    r = open_text(fpath)
    if not r:
        continue
    cwd_ok = False
    try:
        for line in r:
            line = line.strip()
            if not line:
                continue
            try:
                obj = json.loads(line)
            except Exception:
                continue
            ts = obj.get("timestamp", "")
            typ = obj.get("type", "")
            payload = obj.get("payload") or {}
            if typ == "session_meta":
                cwd = (payload.get("cwd") or "") + "|" + "|".join(payload.get("runtime_workspace_roots") or [])
                if PROJECT in cwd:
                    cwd_ok = True
                else:
                    break  # skip whole file if cwd not project
                continue
            if not cwd_ok:
                continue
            if typ == "event_msg" and payload.get("type") == "user_message":
                msg = payload.get("message") or ""
                if msg:
                    record(ts, fpath, "user_message", msg)
            elif typ == "response_item" and payload.get("type") == "message" and payload.get("role") == "user":
                texts = []
                for c in payload.get("content") or []:
                    if isinstance(c, dict) and c.get("type") in ("input_text", "text"):
                        texts.append(c.get("text") or "")
                if texts:
                    record(ts, fpath, "response_user", "\n".join(texts))
    finally:
        r.close()

print(f"extracted user messages: {len(results)}")
with open(OUT, "w", encoding="utf-8") as f:
    for r0 in results:
        f.write(json.dumps(r0, ensure_ascii=False) + "\n")
print("written:", OUT)
