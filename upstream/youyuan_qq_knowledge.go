package main

// 知识库面板：AI 备忘录 / 知识库 / Skill 三类目录 + 上传入库。
//
// 背景：用户点开「知识库与上下文」只看到 not_found。根因不是仓库为空，而是
// youyuan_qq_rag.go 的 yyQQRagRoutes **只定义不注册**（全仓零调用点），于是
// /api/qq/rag/* 落到 main.go:268 handleIndex 的 /api/ 兜底，返回
// {"ok":false,"error":"not_found","path":...}，前端 youyuan_qq_knowledge.js
// 把 stats.error 原样显示成「not_found」。
//
// 本文件做两件事：
//   1. 真正注册 /api/qq/kb/*（tree / doc / text / upload / search）；
//   2. 打开时就有三类目录，每类至少一条示例，用户不必先「造」目录。
//
// 存储复用 youyuan_qq_rag.go 的 bbolt 库（dataDir/qq-rag.db）与分片、倒排、
// 落盘函数——不复制一份存储，将来群聊 RAG 注入接上后，这里的内容天然可被检索。
// 但**不**依赖 yyRagEnabled：那个开关管的是「群聊记忆是否注入」，知识库面板
// 的读写不应该被它关掉（否则用户上传会得到 rag_disabled 这种看不懂的错）。

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	// 单文件上限 4 MiB（zip 按压缩后的字节算）。超过直接拒绝并说清多少 MB。
	yyKBMaxFileBytes = 4 << 20
	// 单篇入库文本上限 1 MiB。超了拒绝，不做静默截断——截断会让用户以为传全了。
	yyKBMaxTextBytes = 1 << 20
	// zip 里最多看 200 个条目，解出的文本合计 ≤ 8 MiB，单条目 ≤ 1 MiB。
	yyKBZipMaxEntries = 200
	yyKBZipMaxTotal   = 8 << 20
	yyKBZipMaxOne     = 1 << 20
	// 预览一次最多吐 32 KiB，避免面板被一篇大文档拖死。
	yyKBPreviewBytes = 32 << 10
	// 每个目录最多列 100 条。
	yyKBListMax = 100
)

// yyKBCategory 一类目录。Writable=false 表示只能看内置内容（Skill 的内置部分）。
type yyKBCategory struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	Writable bool   `json:"writable"`
}

// 三类目录的固定顺序：备忘录 → 知识库 → Skill。
var yyKBCategories = []yyKBCategory{
	{"memo", "AI 备忘录", "AI 自己记的结论、约定和待办。可读可写，回答前会优先翻这里。", true},
	{"kb", "知识库", "用户上传的资料：服务器规则、玩法说明、FAQ、导出的文档。", true},
	{"skill", "Skill", "技能说明。内置 parts 只读，另可加自己的技能笔记。", true},
}

// yyKBTextExts 能直接当文本读的扩展名。zip 内部也按这个白名单挑。
var yyKBTextExts = map[string]bool{
	".md": true, ".markdown": true, ".txt": true, ".json": true, ".csv": true,
	".tsv": true, ".log": true, ".yml": true, ".yaml": true, ".ini": true,
}

var yyKBState = struct {
	sync.Mutex
	Init bool
	Root string
}{}

func yyKBKind(cat string) string {
	switch strings.ToLower(strings.TrimSpace(cat)) {
	case "memo", "记忆", "备忘录":
		return "memo"
	case "skill", "skills", "技能":
		return "skill"
	case "kb", "knowledge", "知识库", "doc", "":
		return "kb"
	}
	return ""
}

// yyKBKindOfDoc 把库里已有的 kind 归到三类目录。
// 老数据的 kind 是 manual/rules/chat/game（RAG 原生词表），统一归「知识库」。
func yyKBKindOfDoc(kind string) string {
	switch kind {
	case "memo", "skill":
		return kind
	}
	return "kb"
}

// yyKBInit 打开知识库库文件。只在 yyRag 还没初始化时打开一次：
// yyRagInit 会重置全部内存表，重复调用会把已载入的文档冲掉。
func yyKBInit(root string) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	yyKBState.Lock()
	defer yyKBState.Unlock()
	if yyKBState.Init {
		return
	}
	yyRag.Lock()
	opened := yyRag.Root != "" || yyRag.DB != nil
	yyRag.Unlock()
	if !opened {
		yyRagInit(root)
	}
	yyKBState.Init, yyKBState.Root = true, root
	yyKBSeed()
}

// yyKBRoot 数据目录：yyRegisterQQ 先设置 yyQQPath 再注册路由，从这里反推。
func yyKBRoot() string {
	if p := strings.TrimSpace(yyQQPath); p != "" {
		return filepath.Dir(p)
	}
	yyKBState.Lock()
	defer yyKBState.Unlock()
	return yyKBState.Root
}

// ---------- 入库 ----------

// yyKBTitle 从正文首行兜底出标题。
func yyKBTitle(title, body string) string {
	title = strings.TrimSpace(title)
	if title != "" {
		return yyQQClip(title, 120)
	}
	if line := strings.TrimSpace(strings.SplitN(body, "\n", 2)[0]); line != "" {
		return yyQQClip(strings.TrimPrefix(line, "#"), 60)
	}
	return "未命名文档"
}

// yyKBNormalize 统一换行、去掉 NUL 等控制字符（二进制混进来会污染检索词表）。
func yyKBNormalize(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\x00", "")
	return strings.TrimSpace(text)
}

// yyKBIsText 粗判文本：有 NUL 或不是合法 UTF-8 就当二进制，不入库。
func yyKBIsText(data []byte) bool {
	head := data
	if len(head) > 8192 {
		head = head[:8192]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return false
	}
	return utf8.Valid(data)
}

// yyKBUpsert 写一篇文档；同名同类先删旧再写新（用户重新上传同一份资料即覆盖）。
func yyKBUpsert(cat, title, source, body string) (id string, chunks int, replaced bool, err error) {
	kind := yyKBKind(cat)
	if kind == "" {
		return "", 0, false, fmt.Errorf("未知目录")
	}
	body = yyKBNormalize(body)
	if body == "" {
		return "", 0, false, fmt.Errorf("内容是空的，没东西可存")
	}
	if len(body) > yyKBMaxTextBytes {
		return "", 0, false, fmt.Errorf("正文 %s 超过单篇 1 MB 上限，请拆成几篇再传", yyKBSize(len(body)))
	}
	title = yyKBTitle(title, body)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(kind+"\x00"+body)))

	yyRag.Lock()
	// 完全相同的正文 + 同类：直接复用，不重复占空间。
	if existed, ok := yyRag.Hashes[hash]; ok {
		if doc, ok := yyRag.Docs[existed]; ok && yyKBKindOfDoc(doc.Kind) == kind {
			yyRag.Unlock()
			return existed, doc.Chunks, false, nil
		}
	}
	// 同名覆盖：同一目录下同名文档只保留最新一份。
	victims := []string{}
	for did, doc := range yyRag.Docs {
		if yyKBKindOfDoc(doc.Kind) == kind && doc.Title == title {
			victims = append(victims, did)
		}
	}
	for _, did := range victims {
		yyRagDeleteDocLocked(did)
	}
	yyRag.Unlock()

	docID := hash[:16]
	rows := []yyRagChunk{}
	seq := 0
	for _, section := range yyRagSplitSections(body) {
		for _, piece := range yyRagWindow(section.Text, yyRagWindowChars, yyRagOverlapChars) {
			rows = append(rows, yyRagChunk{DocID: docID, Text: piece, Heading: section.Heading, Seq: seq, Tokens: yyRagEstimate(piece)})
			seq++
		}
	}
	if len(rows) == 0 {
		return "", 0, false, fmt.Errorf("内容是空的，没东西可存")
	}
	if source == "" {
		source = "panel"
	}
	doc := yyRagDoc{
		ID: docID, Scope: "global", Kind: kind, Title: title, Source: source,
		Hash: hash, CreatedAt: time.Now(), Enabled: true,
	}
	yyRagWriteDoc(doc, rows)
	yyRag.Lock()
	yyRagEvictLocked()
	yyRag.Unlock()
	return docID, len(rows), len(victims) > 0, nil
}

// yyKBSeed 首次打开时补三类示例，幂等：按固定 ID 判存在。
func yyKBSeed() {
	type seed struct{ id, cat, title, source, body string }
	rows := []seed{
		{"kbseed-memo-01", "memo", "备忘 001 · 这个目录怎么用", "seed",
			"## 这里放什么\n" +
				"机器人自己攒下来的结论：群里定过的规矩、玩家的偏好、踩过的坑。\n" +
				"一条一件事，标题写清楚，别写成日记。\n\n" +
				"## 怎么写\n" +
				"用「## 小标题」分段，检索时会按小标题切片，命中后模型看到的就是这一小段。\n" +
				"内容会进机器人的参考资料，所以：不确定的事别写在这里，写「待确认」。\n\n" +
				"## 谁能改\n" +
				"面板上直接编辑保存即可；同样的标题再存一次会覆盖旧的，不会堆两份。"},
		{"kbseed-memo-02", "memo", "备忘 002 · 回答口径（示例，可改写）", "seed",
			"## 语气\n" +
				"群里说话短一点，先说结论，再给一句原因。不堆排比句，不喊口号。\n\n" +
				"## 不确定的时候\n" +
				"直接说不知道，并说明还缺哪个信息。不许编造版本号、命令和数值。\n\n" +
				"## 涉及权限\n" +
				"只有管理员以上的请求才执行改动类操作，其余只解释、不代办。"},
		{"kbseed-kb-01", "kb", "示例 · 服务器规则（可改写）", "seed",
			"## 进服方式\n" +
				"把这一段换成你自己的流程：客户端版本、进服入口、需要先加哪个群。\n\n" +
				"## 禁止事项\n" +
				"外挂、刷物品、卡地形、恶意破坏他人建筑。发现即记录，由管理员处理。\n\n" +
				"## 违规处理\n" +
				"第一次提醒并留档；再次违规按群规处理。处理记录写进「AI 备忘录」，不要只留在聊天里。\n\n" +
				"## 常用问题\n" +
				"问：为什么进不去？答：先看客户端版本，再确认是否在开放时间段。\n" +
				"问：能发坐标吗？答：可以，但要说明是哪个世界、哪个维度。"},
		{"kbseed-skill-01", "skill", "示例 · 自建技能笔记", "seed",
			"## 这是什么\n" +
				"Skill 目录里除了内置的技能说明，还可以放自己写的操作手册：某个插件怎么用、某条命令链的入口在哪。\n\n" +
				"## 写法\n" +
				"标题写技能名；正文按「能做什么 / 怎么调用 / 失败怎么办」三段写，别贴大段源码。\n\n" +
				"## 与知识库的区别\n" +
				"知识库放「事实」（规则、FAQ）；Skill 放「怎么做」（步骤、调用方式）。"},
	}
	now := time.Now()
	for _, s := range rows {
		yyRag.Lock()
		_, ok := yyRag.Docs[s.id]
		yyRag.Unlock()
		if ok {
			continue
		}
		docs := []yyRagChunk{}
		seq := 0
		for _, section := range yyRagSplitSections(s.body) {
			for _, piece := range yyRagWindow(section.Text, yyRagWindowChars, yyRagOverlapChars) {
				docs = append(docs, yyRagChunk{DocID: s.id, Text: piece, Heading: section.Heading, Seq: seq, Tokens: yyRagEstimate(piece)})
				seq++
			}
		}
		if len(docs) == 0 {
			continue
		}
		doc := yyRagDoc{
			ID: s.id, Scope: "global", Kind: s.cat, Title: s.title, Source: s.source,
			Hash: fmt.Sprintf("%x", sha256.Sum256([]byte(s.cat+"\x00"+s.body))),
			CreatedAt: now, Enabled: true,
		}
		yyRagWriteDoc(doc, docs)
	}
}

// ---------- 读模型 ----------

type yyKBItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Kind      string `json:"kind"`
	Source    string `json:"source"`
	Chunks    int    `json:"chunks"`
	Tokens    int    `json:"tokens"`
	Bytes     int    `json:"bytes"`
	Builtin   bool   `json:"builtin"`
	UpdatedAt string `json:"updated_at"`
}

// yyKBItems 列一个目录的条目：内置技能在前，用户文档按更新时间倒序。
func yyKBItems(cat string) []yyKBItem {
	out := []yyKBItem{}
	if cat == "skill" {
		for _, s := range aiSkills {
			out = append(out, yyKBItem{ID: "builtin:" + s.ID, Title: s.Name, Kind: "skill", Source: "内置", Builtin: true})
		}
	}
	yyRag.Lock()
	rows := []yyKBItem{}
	for id, doc := range yyRag.Docs {
		if yyKBKindOfDoc(doc.Kind) != cat {
			continue
		}
		n := 0
		for _, c := range yyRag.Chunks[id] {
			n += len(c.Text)
		}
		rows = append(rows, yyKBItem{
			ID: id, Title: doc.Title, Kind: doc.Kind, Source: doc.Source,
			Chunks: doc.Chunks, Tokens: doc.Tokens, Bytes: n,
			UpdatedAt: doc.UpdatedAt.Format("2006-01-02 15:04"),
		})
	}
	yyRag.Unlock()
	sort.Slice(rows, func(i, j int) bool { return rows[i].UpdatedAt > rows[j].UpdatedAt })
	if len(rows) > yyKBListMax {
		rows = rows[:yyKBListMax]
	}
	return append(out, rows...)
}

func yyKBSize(n int) string {
	if n < 1024 {
		return strconv.Itoa(n) + " B"
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.2f MB", float64(n)/1024/1024)
}

// yyKBReadDoc 取正文。内置技能从内嵌 skills/ 读，其余从库里按 chunk 拼。
func yyKBReadDoc(id string) (title, text string, builtin bool, err error) {
	if strings.HasPrefix(id, "builtin:") {
		skillID := strings.TrimPrefix(id, "builtin:")
		for _, s := range aiSkills {
			if s.ID != skillID {
				continue
			}
			buf := strings.Builder{}
			for _, src := range s.Sources {
				data, e := aiSkillsFS.ReadFile(src)
				if e != nil {
					continue
				}
				buf.WriteString("<!-- " + src + " -->\n")
				buf.Write(data)
				buf.WriteString("\n\n")
				if buf.Len() > yyKBPreviewBytes {
					break
				}
			}
			body := buf.String()
			if body == "" {
				return s.Name, "", true, fmt.Errorf("这个技能没有可读的正文文件")
			}
			return s.Name, yyKBClip(body, yyKBPreviewBytes), true, nil
		}
		return "", "", true, fmt.Errorf("找不到这个内置技能")
	}
	yyRag.Lock()
	defer yyRag.Unlock()
	doc, ok := yyRag.Docs[id]
	if !ok {
		return "", "", false, fmt.Errorf("文档不存在，可能已被删除")
	}
	var buf strings.Builder
	for _, c := range yyRag.Chunks[id] {
		if c.Heading != "" {
			buf.WriteString("## " + c.Heading + "\n")
		}
		buf.WriteString(c.Text)
		buf.WriteString("\n")
		if buf.Len() > yyKBPreviewBytes {
			break
		}
	}
	return doc.Title, yyKBClip(buf.String(), yyKBPreviewBytes), false, nil
}

// yyKBClip 按字符截断（limit 是字符数，不是字节数）。
func yyKBClip(text string, limit int) string {
	r := []rune(text)
	if len(r) <= limit {
		return text
	}
	return string(r[:limit]) + "\n\n…（预览已截断，完整内容在源文件里）"
}

// ---------- 上传 ----------

// yyKBZipPart 从压缩包里抽出来的一份文本。
type yyKBZipPart struct {
	Name string
	Text string
}

// yyKBUnzipText 只把文本挑出来，二进制条目跳过并计数（绝不把二进制塞进上下文）。
func yyKBUnzipText(data []byte) (parts []yyKBZipPart, skipped int, err error) {
	zr, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		return nil, 0, fmt.Errorf("压缩包读不了：不是有效的 zip（%v）", e)
	}
	total, seen := 0, 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || strings.HasPrefix(f.Name, "__MACOSX/") {
			continue
		}
		seen++
		if seen > yyKBZipMaxEntries {
			return parts, skipped, fmt.Errorf("压缩包里超过 %d 个文件，请先挑出需要的再传", yyKBZipMaxEntries)
		}
		if !yyKBTextExts[strings.ToLower(path.Ext(f.Name))] {
			skipped++
			continue
		}
		rc, e := f.Open()
		if e != nil {
			skipped++
			continue
		}
		raw, e := io.ReadAll(io.LimitReader(rc, yyKBZipMaxOne+1))
		rc.Close()
		if e != nil || len(raw) > yyKBZipMaxOne {
			skipped++
			continue
		}
		if !yyKBIsText(raw) {
			skipped++
			continue
		}
		total += len(raw)
		if total > yyKBZipMaxTotal {
			return parts, skipped, fmt.Errorf("压缩包解出的文本超过 %s，请分批传", yyKBSize(yyKBZipMaxTotal))
		}
		parts = append(parts, yyKBZipPart{Name: f.Name, Text: string(raw)})
	}
	return parts, skipped, nil
}

// ---------- 路由 ----------

// yyQQKnowledgeRoutes 知识库面板接口。由 yyQQCapRoutes 调用（见 youyuan_qq_caps.go），
// 这样不用改 youyuan_qq.go。dataDir 从 yyQQPath 反推，yyRegisterQQ 已先赋值。
//
//	GET    /api/qq/kb/tree            三类目录 + 条目 + 限制值
//	GET    /api/qq/kb/doc?id=         预览正文（内置技能用 builtin:<skillID>）
//	DELETE /api/qq/kb/doc?id=         删除（内置不可删）
//	POST   /api/qq/kb/text            粘贴保存 {category,title,text}
//	POST   /api/qq/kb/upload          上传 multipart file + category
//	POST   /api/qq/kb/search          {query,top_k} 试用检索
func yyQQKnowledgeRoutes(mux *http.ServeMux) {
	root := yyKBRoot()
	if root != "" {
		yyKBInit(root)
	}

	mux.HandleFunc("/api/qq/kb/tree", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET required", http.StatusMethodNotAllowed)
			return
		}
		if root == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "数据目录未初始化（yyQQPath 为空）"})
			return
		}
		yyKBInit(root)
		out := make([]map[string]any, 0, len(yyKBCategories))
		for _, c := range yyKBCategories {
			items := yyKBItems(c.ID)
			out = append(out, map[string]any{
				"id": c.ID, "name": c.Name, "desc": c.Desc, "writable": c.Writable,
				"count": len(items), "items": items,
			})
		}
		yyRag.Lock()
		stats := yyRagStatsLocked()
		diskErr := yyRag.DiskError
		yyRag.Unlock()
		writeJSON(w, map[string]any{
			"ok": true, "categories": out, "stats": stats, "persistence_error": diskErr,
			"limits": map[string]any{
				"file_bytes": yyKBMaxFileBytes, "text_bytes": yyKBMaxTextBytes,
				"zip_entries": yyKBZipMaxEntries, "zip_total": yyKBZipMaxTotal,
				"exts": []string{".md", ".txt", ".json", ".csv", ".zip"},
			},
		})
	})

	mux.HandleFunc("/api/qq/kb/doc", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "缺少 id"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			if root == "" {
				writeJSON(w, map[string]any{"ok": false, "error": "数据目录未初始化"})
				return
			}
			yyKBInit(root)
			title, text, builtin, e := yyKBReadDoc(id)
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "id": id, "title": title, "text": text, "builtin": builtin})
		case http.MethodDelete:
			if strings.HasPrefix(id, "builtin:") {
				writeJSON(w, map[string]any{"ok": false, "error": "内置技能说明不能删，只能删自己上传的文档"})
				return
			}
			if root == "" {
				writeJSON(w, map[string]any{"ok": false, "error": "数据目录未初始化"})
				return
			}
			yyKBInit(root)
			n := yyRagDeleteDoc(id)
			if n == 0 {
				writeJSON(w, map[string]any{"ok": false, "error": "文档不存在，可能已被删除"})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "deleted_chunks": n})
		default:
			http.Error(w, "GET/DELETE required", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/qq/kb/text", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		if root == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "数据目录未初始化"})
			return
		}
		var body struct {
			Category string `json:"category"`
			Title    string `json:"title"`
			Text     string `json:"text"`
		}
		if e := json.NewDecoder(io.LimitReader(r.Body, yyKBMaxTextBytes+4096)).Decode(&body); e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "请求格式无效"})
			return
		}
		yyKBInit(root)
		id, chunks, replaced, e := yyKBUpsert(body.Category, body.Title, "panel", body.Text)
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "id": id, "chunks": chunks, "replaced": replaced})
	})

	mux.HandleFunc("/api/qq/kb/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		if root == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "数据目录未初始化"})
			return
		}
		// 上限比单文件多留 64 KiB 给 multipart 头，超了由 MaxBytesReader 直接断。
		r.Body = http.MaxBytesReader(w, r.Body, yyKBMaxFileBytes+64<<10)
		if e := r.ParseMultipartForm(8 << 20); e != nil {
			if strings.Contains(e.Error(), "too large") {
				writeJSON(w, map[string]any{"ok": false, "error": "文件超过 " + yyKBSize(yyKBMaxFileBytes) + " 上限，请拆分或压缩后再传"})
				return
			}
			writeJSON(w, map[string]any{"ok": false, "error": "上传失败：表单格式不对（要 multipart，字段名 file）"})
			return
		}
		file, header, e := r.FormFile("file")
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "没收到文件（表单字段名应为 file）"})
			return
		}
		defer file.Close()
		raw, e := io.ReadAll(io.LimitReader(file, yyKBMaxFileBytes+1))
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "读文件失败"})
			return
		}
		if len(raw) > yyKBMaxFileBytes {
			writeJSON(w, map[string]any{"ok": false, "error": "文件超过 " + yyKBSize(yyKBMaxFileBytes) + " 上限（收到 " + yyKBSize(int(header.Size)) + "），请拆分或压缩后再传"})
			return
		}
		cat := yyKBKind(r.FormValue("category"))
		if cat == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "请先选一个目录：AI 备忘录 / 知识库 / Skill"})
			return
		}
		yyKBInit(root)
		name := filepath.Base(header.Filename)
		ext := strings.ToLower(filepath.Ext(name))

		if ext == ".zip" {
			parts, skipped, e := yyKBUnzipText(raw)
			if e != nil {
				writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
				return
			}
			if len(parts) == 0 {
				writeJSON(w, map[string]any{"ok": false, "error": "压缩包里没找到可入库的文本（只看 .md/.txt/.json/.csv/.yml 等），" + strconv.Itoa(skipped) + " 个条目已跳过"})
				return
			}
			added, failed := []string{}, []string{}
			for _, p := range parts {
				title := strings.TrimSuffix(path.Base(p.Name), path.Ext(p.Name))
				if _, _, _, e := yyKBUpsert(cat, title, "zip:"+name, p.Text); e != nil {
					failed = append(failed, title+"（"+e.Error()+"）")
					continue
				}
				added = append(added, title)
			}
			out := map[string]any{
				"ok": len(added) > 0, "added": len(added), "skipped": skipped,
				"titles": added, "failed": failed,
			}
			if len(added) == 0 {
				out["error"] = "压缩包里的文本都没能入库"
			}
			writeJSON(w, out)
			return
		}

		if !yyKBTextExts[ext] {
			writeJSON(w, map[string]any{"ok": false, "error": "只支持 .md/.txt/.json/.csv/.zip，收到 " + ext})
			return
		}
		if !yyKBIsText(raw) {
			writeJSON(w, map[string]any{"ok": false, "error": "这个文件不是纯文本（含二进制字节），知识库只存文本；如果是文档，请导出成 .md/.txt 再传"})
			return
		}
		title := strings.TrimSuffix(name, filepath.Ext(name))
		id, chunks, replaced, e := yyKBUpsert(cat, title, "upload:"+name, string(raw))
		if e != nil {
			writeJSON(w, map[string]any{"ok": false, "error": e.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "id": id, "title": title, "chunks": chunks, "replaced": replaced, "added": 1})
	})

	mux.HandleFunc("/api/qq/kb/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		if root == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "数据目录未初始化"})
			return
		}
		var body struct {
			Query string `json:"query"`
			TopK  int    `json:"top_k"`
			Cat   string `json:"category"`
		}
		if e := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body); e != nil || strings.TrimSpace(body.Query) == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "先输入要检索的词"})
			return
		}
		yyKBInit(root)
		cf := yyQQSnapshot()
		hits, mode := yyRagSearch(r.Context(), cf, body.Query, body.TopK)
		if cat := yyKBKind(body.Cat); cat != "" && body.Cat != "" {
			kept := []yyRagHit{}
			for _, h := range hits {
				yyRag.Lock()
				doc, ok := yyRag.Docs[h.DocID]
				yyRag.Unlock()
				if ok && yyKBKindOfDoc(doc.Kind) == cat {
					kept = append(kept, h)
				}
			}
			hits = kept
		}
		if hits == nil {
			hits = []yyRagHit{}
		}
		writeJSON(w, map[string]any{"ok": true, "mode": mode, "hits": hits})
	})
}
