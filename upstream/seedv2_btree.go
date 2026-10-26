package main

// ═══════════════════════════════════════════════════════════════════════
// seedv2_btree.go —— 1.18+ 生物群系 climate 参数检索表（数据装载）
//
// 【归属】agent: seed-btree
// 【来源】D:/种子查询器自研/src/btree.mjs（5 张表：btree18 / btree192 /
//         btree19 / btree20 / btree21wd，共 44195 个节点）
// 【做法】不把这 4 万多个节点写成 Go 源码字面量（编译会非常慢）。做法是：
//         1. docs/tools/gen_btree.py 把 btree.mjs 转成 seedv2_btree.dat；
//         2. 用 //go:embed 把 .dat 编进二进制，init 时解析一次；
//         3. 解析失败直接 panic —— 宁可启动就炸，也不能静默返回空表让全图变海洋。
//
// .dat 格式（全部小端，ver 1）：
//   magic   [4]byte = "YYBT"
//   ver     uint16  = 1
//   count   uint16  = 5
//   每张表重复：
//     nameLen uint8 + name []byte
//     order   uint8
//     nsteps  uint8  + steps   []int32   (nsteps<=16)
//     nparam  uint32 + params  []int32   (2*nparam 个，成对 lo,hi)
//     nnodes  uint32 + nodes   []uint64
//
// 只读约定：init 之后再无人写这几张表。返回的是包级共享指针，
// 调用方只许读，不许改 Steps/Param/Nodes 里的元素（改了全局就脏了）。
// ═══════════════════════════════════════════════════════════════════════

import (
	_ "embed"
	"encoding/binary"
	"fmt"
)

//go:embed seedv2_btree.dat
var yyV2BTreeData []byte

const (
	yyV2BTreeMagic    = "YYBT"
	yyV2BTreeVersion  = 1
	yyV2BTreeMaxSteps = 16     // 层距数组上限
	yyV2BTreeMaxParam = 100000 // 参数区间上限
	yyV2BTreeMaxNodes = 200000 // 节点上限
)

// yyV2BTreeOrder 是 /api/seed/manifest 自检时要展示的固定顺序。
var yyV2BTreeOrder = []string{"btree18", "btree192", "btree19", "btree20", "btree21wd"}

var (
	yyV2BTreeTables map[string]*yyV2BTree
	yyV2BTreeNames  []string
)

func init() {
	yyV2BTreeTables, yyV2BTreeNames = yyV2BTreeLoad(yyV2BTreeData)
}

// yyV2BTreeLoad 解析 .dat，任何一处不合法都 panic。
func yyV2BTreeLoad(data []byte) (map[string]*yyV2BTree, []string) {
	fail := func(format string, a ...interface{}) {
		panic("seedv2 btree: " + fmt.Sprintf(format, a...))
	}

	// ---- 头部 ----
	if len(data) < 8 {
		fail("数据太短 %d 字节（连头部都不够）", len(data))
	}
	if string(data[0:4]) != yyV2BTreeMagic {
		fail("magic 不对：%q，应为 %q", data[0:4], yyV2BTreeMagic)
	}
	ver := binary.LittleEndian.Uint16(data[4:6])
	if ver != yyV2BTreeVersion {
		fail("版本不支持：%d，只认 %d", ver, yyV2BTreeVersion)
	}
	count := int(binary.LittleEndian.Uint16(data[6:8]))
	if count <= 0 || count > len(yyV2BTreeOrder) {
		fail("表数量不合理：%d", count)
	}

	off := 8

	// 游标读取，每一步都做剩余长度校验，杜绝越界。
	need := func(n int, what string) {
		if n < 0 || off+n > len(data) {
			fail("%s 越界：需要 %d 字节，剩余 %d（off=%d）", what, n, len(data)-off, off)
		}
	}
	readU8 := func(what string) uint8 {
		need(1, what)
		v := data[off]
		off++
		return v
	}
	readU32 := func(what string) uint32 {
		need(4, what)
		v := binary.LittleEndian.Uint32(data[off : off+4])
		off += 4
		return v
	}

	tables := make(map[string]*yyV2BTree, count)
	names := make([]string, 0, count)

	for i := 0; i < count; i++ {
		// ---- name ----
		nl := int(readU8("nameLen"))
		if nl <= 0 || nl > 32 {
			fail("第 %d 张表名长度不合理：%d", i, nl)
		}
		need(nl, "name")
		name := string(data[off : off+nl])
		off += nl
		if _, dup := tables[name]; dup {
			fail("表名重复：%q", name)
		}

		// ---- order ----
		order := int(readU8("order"))
		if order <= 0 || order > 64 {
			fail("%s: order 不合理 %d", name, order)
		}

		// ---- steps ----
		ns := int(readU8("nsteps"))
		if ns <= 0 || ns > yyV2BTreeMaxSteps {
			fail("%s: nsteps 不合理 %d（上限 %d）", name, ns, yyV2BTreeMaxSteps)
		}
		need(ns*4, "steps")
		steps := make([]int32, ns)
		for k := 0; k < ns; k++ {
			steps[k] = int32(binary.LittleEndian.Uint32(data[off : off+4]))
			off += 4
		}

		// ---- params ----
		np := int(readU32("nparam"))
		if np <= 0 || np > yyV2BTreeMaxParam {
			fail("%s: nparam 不合理 %d（上限 %d）", name, np, yyV2BTreeMaxParam)
		}
		need(np*8, "params")
		params := make([][2]int32, np)
		for k := 0; k < np; k++ {
			params[k][0] = int32(binary.LittleEndian.Uint32(data[off : off+4]))
			params[k][1] = int32(binary.LittleEndian.Uint32(data[off+4 : off+8]))
			off += 8
		}

		// ---- nodes ----
		nn := int(readU32("nnodes"))
		if nn <= 0 || nn > yyV2BTreeMaxNodes {
			fail("%s: nnodes 不合理 %d（上限 %d）", name, nn, yyV2BTreeMaxNodes)
		}
		need(nn*8, "nodes")
		nodes := make([]uint64, nn)
		for k := 0; k < nn; k++ {
			nodes[k] = binary.LittleEndian.Uint64(data[off : off+8])
			off += 8
		}

		// 结构自检：6 个参数字节必须都落在 param 范围内，
		// 否则 npDist 会越界（参考实现里是返回 Infinity，这里是坏数据，必须炸）。
		for k, nd := range nodes {
			for b := 0; b < 6; b++ {
				pidx := int((nd >> (8 * uint(b))) & 0xFF)
				if pidx >= np {
					fail("%s: node[%d] 参数下标 %d 越界（param=%d）", name, k, pidx, np)
				}
			}
		}

		tables[name] = &yyV2BTree{
			Name:  name,
			Order: order,
			Steps: steps,
			Param: params,
			Nodes: nodes,
		}
		names = append(names, name)
	}

	// 尾部必须刚好用完：多出来的字节说明生成脚本和 loader 的格式已经不一致了。
	if off != len(data) {
		fail("尾部有 %d 字节未消费（off=%d, len=%d），格式不一致", len(data)-off, off, len(data))
	}

	// 五张表必须齐，缺一张就说明 .dat 是坏的。缺表比启动炸更危险：
	// 上层按版本挑表，挑不到会静默退回 btree18，玩家看到的地图就是错的。
	for _, want := range yyV2BTreeOrder {
		if tables[want] == nil {
			fail("缺少表 %q（已装载 %v）", want, names)
		}
	}

	// names 按固定版本顺序输出，方便 manifest 自检时对拍。
	ordered := make([]string, 0, len(yyV2BTreeOrder))
	for _, n := range yyV2BTreeOrder {
		if tables[n] != nil {
			ordered = append(ordered, n)
		}
	}
	for _, n := range names {
		if !yyV2BTreeHasName(ordered, n) {
			ordered = append(ordered, n)
		}
	}
	return tables, ordered
}

func yyV2BTreeHasName(list []string, name string) bool {
	for _, s := range list {
		if s == name {
			return true
		}
	}
	return false
}

// yyV2BTreeByName 按名字取表；名字必须是
// "btree18" / "btree192" / "btree19" / "btree20" / "btree21wd"。
// 找不到返回 nil —— 调用方必须能容忍 nil（降级到最保守的 btree18 或直接报错）。
func yyV2BTreeByName(name string) *yyV2BTree {
	if yyV2BTreeTables == nil {
		return nil
	}
	return yyV2BTreeTables[name]
}

// yyV2BTreeList 返回已装载的表名（供 /api/seed/manifest 自检）。
func yyV2BTreeList() []string {
	if len(yyV2BTreeNames) == 0 {
		return nil
	}
	out := make([]string, len(yyV2BTreeNames))
	copy(out, yyV2BTreeNames)
	return out
}
