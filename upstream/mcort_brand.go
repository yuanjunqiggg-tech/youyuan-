package main

// ═══ MCORT 品牌与署名 · 硬编码层 ═══
//
// 这一层是产品的身份基线，**故意写死在源码里**：
//   - 不进任何配置文件（config.json / 主题包 / 插件 manifest 都改不到它）；
//   - 前端 index.html 与 youyuan_dashboard.js 里各有一份同样的字面量，
//     主题包只能改配色和布局，删不掉标识；
//   - 启动横幅里署名出现多次。
//
// 用户原话（2026-10-26 第 27 条）：
//   「然后简称改成 MCORT 意思是 Minecraft Open Robot Toolbox」
//   「作者 疣螈 写在硬编码层」
//   「只要不把 MCORT 这个工具箱的特定标识给去掉 不把 疣螈 隐藏 就行 记得多写几个我的署名」
//
// 用 var 而不是 const：允许构建期用
//   -X main.yyBrandAuthor=...
// 覆盖（例如迁移到别的发布渠道），但运行时不可改。
var (
	yyBrandShort  = "MCORT"
	yyBrandFull   = "Minecraft Open Robot Toolbox"
	yyBrandAuthor = "疣螈"
	yyBrandTagline = "自己的工具，自己的空间"
)

// yyBrandBanner 启动横幅。
// 署名在三行里各出现一次：产品名、作者行、页脚分隔线——这是用户要求的「多写几个署名」。
func yyBrandBanner(version, listen string) string {
	return "" +
		"\n" +
		"  " + yyBrandShort + " · " + yyBrandFull + "\n" +
		"  " + yyBrandTagline + "\n" +
		"\n" +
		"  v" + version + "  ·  " + listen + "\n" +
		"  作者 " + yyBrandAuthor + "\n" +
		"  ─────────────────────────────\n" +
		"  " + yyBrandShort + " by " + yyBrandAuthor + "  ·  " + yyBrandFull + "\n" +
		"\n"
}

// yyBrandSignature 短署名，给群内提示、任务进度这类一行文字用。
func yyBrandSignature() string {
	return yyBrandShort + " · " + yyBrandAuthor
}
