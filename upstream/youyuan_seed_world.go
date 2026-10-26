package main

// 世界种子模块（/api/youyuan/world/seed）
//
// 与「导入/导出」同级：独立路由注册、独立目录条目、独立文档章节。
//
// 数据来源（不伪造）：
//   - 内核在登录 StartGame 包里带 WorldSeed（flowers-for-machines/core/minecraft/conn.go
//     的 handleStartGame → GameData.WorldSeed），这是本机从服务器实际收到的值。
//   - 本框架只走基岩版协议，所以读到的是基岩版（32 位语义）种子；
//     /api/seed/* 那套是 Java 版算法（java.util.Random、Java 1.13+ 结构网格），
//     两者不能混用，因此本接口默认不返回 Java 结构结果，必须显式 edition=java 才做「试算」，
//     并标注为试算，不冒充当前世界的真实结构。
//   - 服务端普遍会把真实种子替换为 0（或直接下发 0）。这种情况下我们照样返回 0，
//     但用 seed_known=false + seed_zero_hint 明确说明「读到了字段，但值不可信」。
//
// 上限与并发：只读内存快照，不加锁链、不做网络请求、不读写文件；单次请求 O(1)（试算除外，
// 试算复用 yySeedStructureScanEx 自带 150ms 预算与 60000 区域上限）。

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// yyWorldSeedSnapshot 是本机内核当前连接的世界身份快照。
type yyWorldSeedSnapshot struct {
	Connected  bool
	Joined     bool
	Server     string
	WorldName  string
	Dimension  int32
	Seed       int64
	SeedFiled  bool // StartGame 包确实到达并解析过（不代表值可信）
	SnapshotAt time.Time
}

// yyWorldSeedRead 在持有 bm.mu 的前提下读取连接上的 GameData。
// 与 youyuan_sense.go 的读取方式保持一致：先取 conn 引用，再读 GameData。
func yyWorldSeedRead() yyWorldSeedSnapshot {
	snap := yyWorldSeedSnapshot{SnapshotAt: time.Now()}
	bm := getBotManagerLocked()
	if bm == nil {
		return snap
	}
	bm.mu.Lock()
	defer bm.mu.Unlock()
	snap.Connected = bm.connected
	snap.Joined = bm.connected && bm.inGame
	snap.Server = bm.serverCode
	conn := bm.conn
	if conn == nil {
		return snap
	}
	gd := conn.GameData()
	snap.WorldName = gd.WorldName
	snap.Dimension = gd.Dimension
	snap.Seed = gd.WorldSeed
	// StartGame 是唯一写入 gameData 的入口；EntityUniqueID/WorldName 任一非零即说明包已解析过。
	// 这里只判断「字段读到过」，值是否可信由 seed_known 与 note 另行说明。
	snap.SeedFiled = gd.EntityUniqueID != 0 || gd.EntityRuntimeID != 0 || gd.WorldName != ""
	return snap
}

// yyWorldSeedFail 返回中文可读错误；参数错误用 400，状态类问题用 200 + ok:false，
// 与 /api/youyuan/* 其余模块保持一致。
func yyWorldSeedFail(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	writeJSON(w, map[string]any{"ok": false, "error": msg})
}

// yyWorldSeedBody 组装成功响应。抽出来是为了让 HTTP 与测试共用同一份字段定义。
func yyWorldSeedBody(snap yyWorldSeedSnapshot, seed int64, seedKnown bool, source string) map[string]any {
	body := map[string]any{
		"ok":             true,
		"seed":           seed,
		"seed_text":      strconv.FormatInt(seed, 10),
		"seed_known":     seedKnown,
		"source":         source,
		"edition":        "bedrock",
		"algorithm":      "java_only_for_calculation",
		"connected":      snap.Connected,
		"joined":         snap.Joined,
		"server":         snap.Server,
		"world_name":     snap.WorldName,
		"dimension":      snap.Dimension,
		"sampled_at":     snap.SnapshotAt.Format(time.RFC3339),
		"precision":      "exact_raw_field",
		"seed_note":      "seed_text 是精确十进制字符串；seed 是 JSON number，超过 2^53 时部分客户端会丢精度，请优先用 seed_text。",
		"note":           "种子取自登录 StartGame 包，是本机实际收到的值；核为基岩版，/api/seed/* 的 Java 算法不适用于该世界。",
		"seed_zero_hint": "服务端常把真实种子替换为 0；seed=0 时应视为未知，不要当成真实世界种子。",
		// 版本号默认取内核当前协议版本（handleBotConnect 设置的 1.21.120）。
		// 用户 2026-10-26 第 7 条要求：读主机器人当前种子也要注册成 API，版本号先默认成现在的版本。
		"version":        "1.21.120",
		"version_source": "kernel_default",
	}
	if !seedKnown {
		body["error"] = "尚未从内核读取到世界种子：请先连接服务器并完成登录（StartGame 包到达后才有值），或显式传 seed 参数。"
		body["ok"] = false
	}
	return body
}

// yyWorldSeedRoutes 注册 /api/youyuan/world/seed。
//
// 参数：
//
//	seed      可选，显式指定种子（不依赖连接）。与内核读到的值互斥，显式值优先。
//	structure 可选，结构标识：village/fortress/bastion/monument/mansion/temple。
//	          只有同时给 edition=java 才会做「Java 算法试算」，否则报错说明算法不适用。
//	edition   可选，只有 java 有意义（基岩版结构算法尚未实现）。
//	from_x/from_z/to_x/to_z  试算范围（区块），默认 -256..256。
//	lod       试算分级细节 0/1/2，默认 0。
func yyWorldSeedRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/youyuan/world/seed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method != http.MethodGet {
			yyWorldSeedFail(w, http.StatusMethodNotAllowed, "世界种子查询仅支持 GET")
			return
		}

		snap := yyWorldSeedRead()

		// 1. 先定种子：显式 seed 优先，否则用内核读到的字段。
		explicit, hasExplicit, err := yySeedParamInt64(r, "seed")
		if err != nil {
			yyWorldSeedFail(w, http.StatusBadRequest, err.Error())
			return
		}

		seed := snap.Seed
		seedKnown := false
		source := "kernel"
		switch {
		case hasExplicit:
			seed = explicit
			seedKnown = true
			source = "query"
		case snap.SeedFiled:
			// 读到了字段就算 known；值是否为真实种子由 note / seed_zero_hint 说明。
			seedKnown = true
		}

		// 2. 可选：结构试算。必须显式声明 edition=java，避免把 Java 结果当成基岩世界结论。
		structureID := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("structure")))
		if structureID == "" {
			structureID = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
		}

		if structureID == "" {
			writeJSON(w, yyWorldSeedBody(snap, seed, seedKnown, source))
			return
		}

		edition := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("edition")))
		if edition != "java" {
			yyWorldSeedFail(w, http.StatusBadRequest,
				"内核读到的是基岩版世界种子，Java 版结构算法不适用于该世界；若只想按 Java 算法试算，请显式加 edition=java")
			return
		}
		if !seedKnown {
			yyWorldSeedFail(w, http.StatusConflict,
				"尚未从内核读取到世界种子，无法试算；请先连接服务器，或显式传 seed 参数")
			return
		}

		spec, found := yySeedStructureSpecByID(structureID)
		if !found {
			yyWorldSeedFail(w, http.StatusBadRequest,
				"不支持的结构标识："+structureID+"（可选 village/fortress/bastion/monument/mansion/temple）")
			return
		}
		fromX, err := yySeedParamInt(r, "from_x", -256)
		if err != nil {
			yyWorldSeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		fromZ, err := yySeedParamInt(r, "from_z", -256)
		if err != nil {
			yyWorldSeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		toX, err := yySeedParamInt(r, "to_x", 256)
		if err != nil {
			yyWorldSeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		toZ, err := yySeedParamInt(r, "to_z", 256)
		if err != nil {
			yyWorldSeedFail(w, http.StatusBadRequest, err.Error())
			return
		}
		if toX-fromX > yySeedMaxChunkRange || toZ-fromZ > yySeedMaxChunkRange ||
			fromX-toX > yySeedMaxChunkRange || fromZ-toZ > yySeedMaxChunkRange {
			yyWorldSeedFail(w, http.StatusBadRequest, "查询范围过大：单轴区块跨度不能超过 4096")
			return
		}
		lod, err := yySeedParamLOD(r)
		if err != nil {
			yyWorldSeedFail(w, http.StatusBadRequest, err.Error())
			return
		}

		res := yySeedStructureScanEx(seed, spec, fromX, fromZ, toX, toZ, lod, 0, time.Now().Add(yySeedQueryBudget))
		body := yyWorldSeedBody(snap, seed, seedKnown, source)
		body["java_preview"] = map[string]any{
			"structure":        spec.ID,
			"label":            spec.Label,
			"chunks":           res.Chunks,
			"count":            len(res.Chunks),
			"truncated":        res.Truncated,
			"lod":              res.LOD,
			"sampled":          res.Sampled,
			"skipped_by_lod":   res.SkippedByLOD,
			"degraded":         res.Degraded,
			"reason":           res.Reason,
			"precision":        "candidate",
			"applies_to_world": false,
			"note":             "这是用 Java 版结构算法对同一个数字做的试算，不是当前基岩世界里的真实结构位置；实际位置必须实测。",
		}
		body["precision"] = "exact_raw_field+candidate_preview"
		writeJSON(w, body)
	})
}
