package main

// seedv2_render_shim.go —— 渲染层与图标层之间的过渡垫片
//
// 背景：seedv2_render.go 由 agent 分段写入，其中 yyV2RenderBakeIcons 引用了
// yyV2RenderDefaultIconIDs（该维度默认要画哪些结构的图标）。为了让「任何时刻
// go build 都是绿的」，这个 helper 单独放在这里，不和两个大文件抢写入。
//
// 语义很简单：没有显式指定 types 时，按维度给一套默认结构。这套默认值和
// HTTP 路由层 / 插件 SDK 用的是同一张表（yySeedV2DefaultTypes），不另立一份。

// yyV2RenderDefaultIconIDs 返回某维度默认绘制的结构 id（返回副本，调用方可改）。
// 会按「该版本是否真的有这个结构」过滤一遍：比如 1.15 没有海底废墟，
// 就不该在默认图标集里占位。
func yyV2RenderDefaultIconIDs(dim string, versionCode int32, edition string) []string {
	dim = yyV2NormalizeDimension(dim)
	base := yySeedV2DefaultTypes[dim]
	out := make([]string, 0, len(base))
	seen := map[string]bool{}
	for _, id := range base {
		info, ok := yyV2StructureInfoByID(id, versionCode, edition)
		if !ok || info.Dim != dim || seen[info.ID] {
			continue
		}
		seen[info.ID] = true
		out = append(out, info.ID)
	}
	if len(out) == 0 {
		// 默认集整批不可用（极旧版本）时，退化成「该版本该维度全部可用结构」，
		// 最多 8 种，避免一个图标都不画。
		for _, info := range yyV2StructureList(versionCode, edition) {
			if info.Dim != dim || seen[info.ID] {
				continue
			}
			seen[info.ID] = true
			out = append(out, info.ID)
			if len(out) >= 8 {
				break
			}
		}
	}
	return out
}
