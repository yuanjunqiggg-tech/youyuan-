"""Project-scoped recall: original user records, evidence, local FTS and embeddings.

Only reads explicitly scoped Codex message records; never imports reasoning/tool output.
Credential-bearing originals live in a separate Windows DPAPI vault, outside retrieval.
"""
from __future__ import annotations

import argparse
import ctypes
import hashlib
import json
import os
import re
import sqlite3
import sys
from contextlib import closing
from datetime import datetime, timedelta, timezone
from pathlib import Path

TZ = timezone(timedelta(hours=8))
MODEL = "BAAI/bge-small-zh-v1.5"
MEMORY_HOME = Path("D:/游园/游园数据记忆库")
PRIVATE = Path("D:/游园/私有凭据/continuity")
MODEL_CACHE = "D:/游园/运行环境/向量模型"


def now():
    return datetime.now(TZ).isoformat(timespec="seconds")


def digest(value):
    return hashlib.sha256(value.encode("utf8") if isinstance(value, str) else value).hexdigest()


def protected(data: bytes, decrypt=False):
    from ctypes import wintypes

    class Blob(ctypes.Structure):
        _fields_ = [("length", wintypes.DWORD), ("data", ctypes.POINTER(ctypes.c_ubyte))]

    backing = ctypes.create_string_buffer(data)
    source = Blob(len(data), ctypes.cast(backing, ctypes.POINTER(ctypes.c_ubyte)))
    target = Blob()
    fn = ctypes.windll.crypt32.CryptUnprotectData if decrypt else ctypes.windll.crypt32.CryptProtectData
    fn.argtypes = [ctypes.POINTER(Blob), ctypes.c_void_p, ctypes.c_void_p,
                   ctypes.c_void_p, ctypes.c_void_p, wintypes.DWORD, ctypes.POINTER(Blob)]
    fn.restype = wintypes.BOOL
    if not fn(ctypes.byref(source), None, None, None, None, 1, ctypes.byref(target)):
        raise ctypes.WinError()
    try:
        return ctypes.string_at(target.data, target.length)
    finally:
        ctypes.windll.kernel32.LocalFree(target.data)


def databases(project):
    home = MEMORY_HOME
    (home / "检索").mkdir(parents=True, exist_ok=True)
    PRIVATE.mkdir(parents=True, exist_ok=True)
    db = sqlite3.connect(home / "检索/knowledge.sqlite3", timeout=30)
    db.row_factory = sqlite3.Row
    db.execute("PRAGMA journal_mode=WAL")
    db.execute("PRAGMA busy_timeout=30000")
    db.executescript("""
    CREATE TABLE IF NOT EXISTS messages(
      id TEXT PRIMARY KEY, at TEXT, role TEXT, kind TEXT, text TEXT, body_sha256 TEXT);
    CREATE TABLE IF NOT EXISTS origins(
      message_id TEXT, file TEXT, line INTEGER, thread TEXT,
      PRIMARY KEY(message_id,file,line));
    CREATE TABLE IF NOT EXISTS assets(
      message_id TEXT, path TEXT, kind TEXT, exists_at_import INTEGER,
      PRIMARY KEY(message_id,path));
    CREATE TABLE IF NOT EXISTS cursors(file TEXT PRIMARY KEY, offset INTEGER, line INTEGER);
    CREATE TABLE IF NOT EXISTS source_usage(source TEXT PRIMARY KEY, uses INTEGER, last_used TEXT);
    CREATE TABLE IF NOT EXISTS chunks(
      id TEXT PRIMARY KEY, source TEXT, line INTEGER, role TEXT, text TEXT,
      sha256 TEXT, vector BLOB, model TEXT);
    CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(id UNINDEXED, text);
    """)
    vault = sqlite3.connect(PRIVATE / "private.sqlite3", timeout=30)
    vault.execute("PRAGMA journal_mode=WAL")
    vault.executescript("""
    CREATE TABLE IF NOT EXISTS originals(id TEXT PRIMARY KEY, body_dpapi BLOB, sha256 TEXT);
    CREATE TABLE IF NOT EXISTS credentials(name TEXT PRIMARY KEY, value_dpapi BLOB, source TEXT, checked_at TEXT);
    """)
    return home, db, vault


def credential_setup(vault):
    path = Path("D:/YouYuan-APK/private/linux-access.json")
    secret_values = []
    if path.exists():
        data = json.loads(path.read_text("utf8"))
        for key in ("host", "username", "password", "host_key_sha256"):
            if key in data:
                raw = str(data[key]).encode("utf8")
                sealed = protected(raw)
                assert protected(sealed, True) == raw
                vault.execute("INSERT OR REPLACE INTO credentials VALUES(?,?,?,?)",
                              ("linux." + key, sealed, str(path), now()))
                if key == "password" and data[key]:
                    secret_values.append(str(data[key]))
        vault.commit()
    return secret_values


def redact(text, secrets=()):
    for value in sorted(secrets, key=len, reverse=True):
        text = text.replace(value, "[私有凭据，见DPAPI库]")
    text = re.sub(r"(?i)(bearer\s+|yy//)[a-z0-9._~+/=-]{16,}", r"\1[私有凭据]", text)
    text = re.sub(r"(?i)((?:password|密码|api[_ -]?key|access[_ -]?token)\s*[:：=]\s*[\"']?)[^\s\"'<>]{6,}",
                  r"\1[私有凭据]", text)
    text = re.sub(r"(密码\s+)[A-Za-z0-9!@#$%^&*()_+./=-]{6,}", r"\1[私有凭据]", text)
    text = re.sub(r"\bsk-[A-Za-z0-9_-]{20,}", "[私有凭据]", text)
    return text


def scope_ok(cwd, root):
    cwd = str(cwd).replace("\\", "/").casefold().rstrip("/")
    root = str(root).replace("\\", "/").casefold().rstrip("/")
    return cwd == root or cwd.startswith(root + "/")


def message_content(payload):
    return "\n".join(x.get("text", "") for x in payload.get("content", [])
                     if x.get("type") in ("input_text", "output_text", "text"))


def asset_paths(text):
    result = set(re.findall(r'(?:path=["\'])([A-Za-z]:[/\\][^"\']+)', text))
    result.update(re.findall(r"(?m)^##\s+[^\r\n]+:\s*([A-Za-z]:[/\\][^\r\n]+)", text))
    result.update(re.findall(r"(?m)^\s*(?:[A-Za-z][^\r\n]*?文件[：:]\s*)?([A-Za-z]:[/\\][^\r\n<>\"|]+\.(?:apk(?:\([^)]*\))?(?:\.\d+)?|jpg|jpeg|png|webp|mp4|txt|zip|nbt|mcstructure))\s*$", text))
    return sorted(x.strip() for x in result)


def sync(project, since):
    home, db, vault = databases(project)
    secrets = credential_setup(vault)
    root = project.parent.resolve()
    start = datetime.fromisoformat(since).replace(tzinfo=TZ)
    sessions = Path("C:/Users/Lenovo/.codex/sessions")
    evidence = []
    counts = {"imported_user": 0, "imported_assistant": 0, "malformed_records": 0}
    for file in sessions.rglob("*.jsonl"):
        if file.stat().st_mtime < start.timestamp():
            continue
        with file.open("rb") as stream:
            first = stream.readline()
            try:
                meta = json.loads(first).get("payload", {})
            except (ValueError, AttributeError):
                continue
            if not scope_ok(meta.get("cwd", ""), root):
                continue
            previous = db.execute("SELECT offset,line FROM cursors WHERE file=?", (str(file),)).fetchone()
            line_number = 1
            if previous and previous[0] <= file.stat().st_size:
                stream.seek(previous[0]); line_number = previous[1]
            begin = stream.tell()
            while True:
                offset = stream.tell()
                raw = stream.readline()
                if not raw:
                    break
                if not raw.endswith(b"\n"):
                    stream.seek(offset)
                    break
                line_number += 1
                try:
                    item = json.loads(raw)
                except ValueError:
                    counts["malformed_records"] += 1
                    continue
                payload = item.get("payload", {})
                if item.get("type") != "response_item" or payload.get("type") != "message":
                    continue
                role = payload.get("role")
                if role not in ("user", "assistant") or payload.get("channel") in ("analysis", "summary"):
                    continue
                at = item.get("timestamp", "")
                if not at or datetime.fromisoformat(at.replace("Z", "+00:00")) < start:
                    continue
                text = message_content(payload)
                if not text:
                    continue
                kind = "context" if role == "user" and (text.startswith("# AGENTS.md instructions") or text.startswith("<environment_context>")) else "message"
                key = payload.get("id") or digest(meta.get("id", "") + at + role + text)
                old = db.execute("SELECT 1 FROM messages WHERE id=?", (key,)).fetchone()
                if not old:
                    db.execute("INSERT INTO messages VALUES(?,?,?,?,?,?)",
                               (key, at, role, kind, redact(text, secrets), digest(text)))
                    # Exact original including attachment metadata is recoverable on this Windows account.
                    sealed = protected(json.dumps(payload, ensure_ascii=False).encode("utf8"))
                    vault.execute("INSERT OR IGNORE INTO originals VALUES(?,?,?)", (key, sealed, digest(text)))
                    counts["imported_" + role] += 1
                db.execute("INSERT OR IGNORE INTO origins VALUES(?,?,?,?)",
                           (key, str(file), line_number, meta.get("id", "")))
                for path in asset_paths(text):
                    suffix = Path(path).suffix.lower()
                    kind_asset = "image" if suffix in (".jpg", ".jpeg", ".png", ".webp") else "file"
                    db.execute("INSERT OR IGNORE INTO assets VALUES(?,?,?,?)",
                               (key, path, kind_asset, int(Path(path).exists())))
            db.execute("INSERT OR REPLACE INTO cursors VALUES(?,?,?)", (str(file), stream.tell(), line_number))
            evidence.append({"file": str(file), "thread": meta.get("id"), "bytes_read": stream.tell() - begin,
                             "offset": stream.tell(), "last_line": line_number})
    db.commit(); vault.commit()
    render(project, db)
    report = {"at": now(), "since_beijing": start.isoformat(), "scope": str(root),
              "matched_session_files": evidence, **counts,
              "totals": dict(db.execute("SELECT role,count(*) FROM messages GROUP BY role").fetchall()),
              "assets": db.execute("SELECT count(*) FROM assets").fetchone()[0],
              "limits": ["只保证上述实际取得的项目原始记录；不补造删除/缺失历史", "不导入模型分析、工具输出或其他AI会话", "附件路径只记录存在性，不冒充已读图像或已分析文件", "明文展示脱敏；完整原文及凭据仅在Windows DPAPI私有SQL中"]}
    (home / "coverage.json").write_text(json.dumps(report, ensure_ascii=False, indent=2), "utf8")
    print(json.dumps(report, ensure_ascii=False))


def render(project, db):
    home = MEMORY_HOME
    directory = home / "用户对话"; directory.mkdir(parents=True, exist_ok=True)
    days = {}
    for row in db.execute("SELECT * FROM messages WHERE role='user' ORDER BY at,id"):
        day = datetime.fromisoformat(row["at"].replace("Z", "+00:00")).astimezone(TZ).strftime("%Y-%m-%d")
        origin = db.execute("SELECT file,line FROM origins WHERE message_id=? ORDER BY file LIMIT 1", (row["id"],)).fetchone()
        days.setdefault(day, []).append(f"\n## {row['at']} · {row['id']} · {row['kind']}\n\n来源：{origin['file']}:{origin['line']}\n\n<user-record>\n{row['text']}\n</user-record>\n")
    for day, messages in days.items():
        (directory / (day + ".md")).write_text("# 用户原话与原始附件引用\n\n原文保留接收时的自动上下文标记；自动界面信息、历史引文不等于用户新指令。凭据处脱敏，完整原文在私有DPAPI库。\n" + "".join(messages), "utf8")
    assets = ["# 用户附件路径\n\n记录用户提供的路径及关联消息；存在不代表已解析，不复制到云端。\n"]
    for row in db.execute("SELECT * FROM assets ORDER BY message_id,path"):
        assets.append(f"- `{row['path']}` · {row['kind']} · 导入时存在={bool(row['exists_at_import'])} · 消息={row['message_id']}\n")
    (home / "用户对话/附件路径.md").write_text("".join(assets), "utf8")
    ledger = json.loads((project / "需求逐项总清单.json").read_text("utf8"))
    tasks = ["# 任务清单（由同一需求总账生成，不是另一套状态源）\n\n先做B03/B08命令链与宿主性能；停止实服加压约束仍有效。旧任务服从后续纠正、撤回和当前授权。\n"]
    for task in ledger["requirements"]:
        closed = task["status"].startswith("已完成")
        tasks.append(f"- [{'x' if closed else ' '}] **{task['id']} / {task.get('priority','')}** {task['request']}\n  - 状态：{task['status']}；{task.get('verification','')}\n  - 做法/证据：{task.get('stage','')}\n  - 剩余：{task.get('remaining','')}\n")
    (home / "任务清单.md").write_text("\n".join(tasks), "utf8")


def record(project, file, agent):
    """Other native clients explicitly hand off a message; never scan their storage."""
    file = file.resolve()
    allowed = MEMORY_HOME.parent / "输入" / agent
    if not file.is_relative_to(allowed.resolve()) or file.stat().st_size > 4 * 1024 * 1024:
        raise ValueError("Record must be a <=4MiB JSON file in D:/游园/输入/<agent>/")
    value = json.loads(file.read_text("utf-8-sig"))
    text, at, role = value.get("text"), value.get("at"), value.get("role", "user")
    if not isinstance(text, str) or not text or role not in ("user", "assistant"):
        raise ValueError("Record needs nonempty original text and user/assistant role")
    parsed = datetime.fromisoformat(at.replace("Z", "+00:00"))
    if parsed.tzinfo is None:
        raise ValueError("Record timestamp needs timezone")
    home, db, vault = databases(project)
    with closing(db), closing(vault):
        secrets = credential_setup(vault)
        key = agent + ":" + str(value.get("id") or digest(at + role + text))
        old = db.execute("SELECT body_sha256 FROM messages WHERE id=?", (key,)).fetchone()
        if old and old[0] != digest(text):
            raise ValueError("Original message is immutable; use a new id for a correction")
        db.execute("INSERT OR IGNORE INTO messages VALUES(?,?,?,?,?,?)",
                   (key, at, role, "message", redact(text, secrets), digest(text)))
        db.execute("INSERT OR IGNORE INTO origins VALUES(?,?,?,?)", (key, str(file), 1, agent))
        vault.execute("INSERT OR IGNORE INTO originals VALUES(?,?,?)",
                      (key, protected(json.dumps(value, ensure_ascii=False).encode("utf8")), digest(text)))
        paths = set(asset_paths(text))
        for attachment in value.get("attachments", []):
            path = attachment.get("path") if isinstance(attachment, dict) else attachment
            if not isinstance(path, str) or not re.match(r"^[A-Za-z]:[/\\]", path):
                raise ValueError("Attachment path must be an absolute Windows path")
            paths.add(path)
        for path in paths:
            db.execute("INSERT OR IGNORE INTO assets VALUES(?,?,?,?)", (key, path, "native-client-attachment", int(Path(path).exists())))
        vault.commit(); db.commit(); render(project, db)
        print(json.dumps({"recorded": key, "new": old is None, "actor_label": agent, "native_client_identity_verified": False}, ensure_ascii=False))


def split_text(text, limit=480, overlap=80):
    for start in range(0, len(text), limit - overlap):
        yield start, text[start:start + limit]


def index(project, vectors):
    home, db, _ = databases(project)
    docs = []
    for row in db.execute("SELECT * FROM messages WHERE kind='message'"):
        origin = db.execute("SELECT file,line FROM origins WHERE message_id=? LIMIT 1", (row["id"],)).fetchone()
        docs.append(("message:" + row["id"], origin["file"], origin["line"], row["role"], row["text"]))
    sources = [home / "备忘录.md", home / "任务清单.md", project / "协作交接/维护入口.md"]
    for directory in (home / "经验", home / "日志", home / "资料库"):
        if directory.exists():
            sources.extend(directory.rglob("*.md"))
    for file in sources:
        if file.exists():
            docs.append(("file:" + str(file), str(file), 1, "evidence", file.read_text("utf8")))
    desired = set()
    for key, source, line, role, text in docs:
        for start, body in split_text(text):
            cid = digest(key + ":" + str(start))
            desired.add(cid); sha = digest(body)
            previous = db.execute("SELECT sha256 FROM chunks WHERE id=?", (cid,)).fetchone()
            if previous and previous[0] == sha:
                continue
            db.execute("INSERT OR REPLACE INTO chunks VALUES(?,?,?,?,?,?,NULL,NULL)",
                       (cid, source + ("#" + key if key.startswith("message:") else ""), line, role, body, sha))
    for row in db.execute("SELECT id FROM chunks").fetchall():
        if row[0] not in desired:
            db.execute("DELETE FROM chunks WHERE id=?", (row[0],))
    db.execute("DELETE FROM chunks_fts")
    db.executemany("INSERT INTO chunks_fts VALUES(?,?)", [(row[0], tokens(row[1])) for row in db.execute("SELECT id,text FROM chunks")])
    db.commit()
    if vectors:
        from fastembed import TextEmbedding
        model = TextEmbedding(MODEL, cache_dir=MODEL_CACHE, threads=2)
        rows = db.execute("SELECT id,text FROM chunks WHERE vector IS NULL OR model!=?", (MODEL,)).fetchall()
        for start in range(0, len(rows), 32):
            batch = rows[start:start + 32]
            for row, vector in zip(batch, model.embed([x[1] for x in batch], batch_size=32)):
                db.execute("UPDATE chunks SET vector=?,model=? WHERE id=?", (vector.astype("float32").tobytes(), MODEL, row[0]))
            db.commit()
            print(json.dumps({"embedded": min(start + 32, len(rows)), "pending_at_start": len(rows)}), flush=True)
    print(json.dumps(status(db), ensure_ascii=False))


def tokens(text):
    latin = re.findall(r"[a-z0-9_]+", text.lower())
    chinese = re.findall(r"[\u3400-\u9fff]+", text)
    return " ".join(latin + [segment[i:i + 2] for segment in chinese for i in range(max(1, len(segment) - 1))])


def search(project, query, top, semantic):
    _, db, _ = databases(project)
    terms = list(dict.fromkeys(tokens(query).split()))[:48]
    found, scores = {}, {}
    if terms:
        match = " OR ".join('"' + x.replace('"', '""') + '"' for x in terms)
        rows = db.execute("SELECT c.* FROM chunks_fts f JOIN chunks c ON c.id=f.id WHERE chunks_fts MATCH ? ORDER BY bm25(chunks_fts) LIMIT 50", (match,)).fetchall()
        for rank, row in enumerate(rows):
            found[row["id"]] = row; scores[row["id"]] = 1 / (60 + rank)
    if semantic:
        import numpy as np
        from fastembed import TextEmbedding
        model = TextEmbedding(MODEL, cache_dir=MODEL_CACHE, threads=2)
        q = next(model.query_embed(query))
        rows = db.execute("SELECT * FROM chunks WHERE vector IS NOT NULL AND model=?", (MODEL,)).fetchall()
        if not rows:
            raise RuntimeError("No real vectors indexed; run index --vectors before semantic search")
        matrix = np.stack([np.frombuffer(row["vector"], dtype=np.float32) for row in rows])
        norm = np.maximum(np.linalg.norm(matrix, axis=1) * np.linalg.norm(q), 1e-12)
        sims = matrix @ q / norm
        for rank, i in enumerate(np.argsort(-sims)[:50]):
            row = rows[int(i)];found[row["id"]] = row
            scores[row["id"]] = scores.get(row["id"], 0) + 1 / (60 + rank)
    result = [{"source": found[k]["source"], "line": found[k]["line"], "role": found[k]["role"],
               "score": value, "text": found[k]["text"]} for k, value in sorted(scores.items(), key=lambda x: -x[1])[:top]]
    for item in result:
        db.execute("INSERT INTO source_usage VALUES(?,1,?) ON CONFLICT(source) DO UPDATE SET uses=uses+1,last_used=excluded.last_used", (item["source"], now()))
    db.commit()
    print(json.dumps({"query": query, "mode": "local-bge+fts-rrf" if semantic else "fts", "hits": result}, ensure_ascii=False, indent=2))


def status(db):
    return {"messages": db.execute("SELECT count(*) FROM messages").fetchone()[0],
            "chunks": db.execute("SELECT count(*) FROM chunks").fetchone()[0],
            "real_vectors": db.execute("SELECT count(*) FROM chunks WHERE vector IS NOT NULL AND model=?", (MODEL,)).fetchone()[0],
            "embedding_model": MODEL}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--project", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--agent", choices=("codex", "deepseek-harness", "workbuddy"), default="codex")
    sub = parser.add_subparsers(dest="command", required=True)
    sync_args = sub.add_parser("sync");sync_args.add_argument("--since", default="2026-10-24")
    idx = sub.add_parser("index");idx.add_argument("--vectors", action="store_true")
    query = sub.add_parser("search");query.add_argument("query");query.add_argument("--top", type=int, default=5);query.add_argument("--semantic", action="store_true")
    sub.add_parser("status")
    rec = sub.add_parser("record");rec.add_argument("file", type=Path)
    args = parser.parse_args();project = args.project.resolve()
    success = False
    try:
        if args.command == "sync":sync(project, args.since)
        elif args.command == "index":index(project, args.vectors)
        elif args.command == "search":search(project, args.query, args.top, args.semantic)
        elif args.command == "record":record(project, args.file, args.agent)
        else:print(json.dumps(status(databases(project)[1]), ensure_ascii=False))
        success = True
    finally:
        log = MEMORY_HOME / "日志/操作流水.jsonl"
        log.parent.mkdir(parents=True, exist_ok=True)
        record = {"at": now(), "agent": args.agent, "operation": args.command,
                  "project": str(project), "pid": os.getpid(), "success": success}
        fd = os.open(log, os.O_WRONLY | os.O_CREAT | os.O_APPEND)
        try:os.write(fd, (json.dumps(record, ensure_ascii=False) + "\n").encode("utf8"))
        finally:os.close(fd)


if __name__ == "__main__":
    main()
