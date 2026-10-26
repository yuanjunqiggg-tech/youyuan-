package main

// ═══════════════════════════════════════════════════════════════════════
// seedv2_btree_test.go —— 生物群系参数表装载对拍
//
// 【归属】agent: seed-btree
// 【真值来源】D:/种子查询器自研/src/btree.mjs（用 Node 直接 import 打印）
//   生成真值的命令（可复查）：
//     cd D:/种子查询器自研/src
//     node -e "import('./btree.mjs').then(m=>{
//       for (const k of ['btree18','btree192','btree19','btree20','btree21wd']) {
//         const t=m[k];
//         console.log(k,t.order,t.steps.length,t.param.length,t.nodes.length,
//           t.nodes.slice(0,10).map(x=>x.toString(16).padStart(16,'0')).join(' '));
//       }})"
//   指纹 = sha256(nodes 十六进制以 ',' 连接 + '|' + params "lo:hi" 以 ',' 连接
//                + '|' + steps 以 ',' 连接)
// ═══════════════════════════════════════════════════════════════════════

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"testing"
)

type yyV2BTreeWant struct {
	name     string
	order    int
	steps    []int32
	nSteps   int
	nParams  int
	nNodes   int
	head10   []string // 前 10 个节点的 16 位十六进制（小端解出的 uint64）
	tail3    []string // 最后 3 个节点
	finger   string   // 全量数据指纹（与 Node 侧一致）
}

var yyV2BTreeWants = []yyV2BTreeWant{
	{
		name:    "btree18",
		order:   10,
		steps:   []int32{1111, 111, 11, 1, 0},
		nSteps:  5,
		nParams: 114,
		nNodes:  8421,
		head10: []string{
			"00014f4f4f4f4f4f", "0002164f16021616", "0003164f16021616", "00042c4f3d411645",
			"ff042c4f3d414a44", "ff1b2c4f3d415a44", "ff012c4f3d413544", "ff042c4f3d414a5c",
			"ff152c4f3d415a5c", "ff232c4f3d41355c",
		},
		tail3:  []string{"ff02707169406369", "ff02707167536369", "ff026d7169436369"},
		finger: "619be3c2bd0d40d349b876b1d71e6ee5cf4b54fc60022eef43a2bf819fbe30b3",
	},
	{
		name:    "btree192",
		order:   10,
		steps:   []int32{1111, 111, 11, 1, 0},
		nSteps:  5,
		nParams: 107,
		nNodes:  8438,
		head10: []string{
			"0001494949494949", "0002134b13011313", "0003134a13011313", "0004134a13031313",
			"ff0a134913201307", "ff31134913021322", "ff0b47492d3d1307", "ff32134913021307",
			"ff0a136913201307", "ff0b47692d3d1307",
		},
		tail3:  []string{"ff026869603a4460", "ff026869603a5360", "ff026869603a5b60"},
		finger: "8d794c92e1af235b3ee3ea61019e658d243fa7ba264218418836ea155659e273",
	},
	{
		name:    "btree19",
		order:   6,
		steps:   []int32{1555, 259, 43, 7, 1, 0},
		nSteps:  6,
		nParams: 139,
		nNodes:  9112,
		head10: []string{
			"0001616161616161", "00021c621c021c1c", "00031c621c021c1c", "00041c621c051c1c",
			"0005496251521456", "ff10496151525c56", "ff10496151524556", "ff10496151520d56",
			"ff10498851525c56", "ff10498851524556",
		},
		tail3:  []string{"ff0287887f525c7f", "ff0287887f526e7f", "ff0287887f52787f"},
		finger: "9d226133f63e1f9535ba780760bd351d1a7a0c519003a9e6faeed336952cf341",
	},
	{
		name:    "btree20",
		order:   6,
		steps:   []int32{1555, 259, 43, 7, 1, 0},
		nSteps:  6,
		nParams: 139,
		nNodes:  9112,
		head10: []string{
			"0001616161616161", "00021c621c021c1c", "00031c621c021c1c", "00041c621c051c1c",
			"0005496251521456", "ff10496151525c56", "ff10496151524556", "ff10496151520d56",
			"ff10498851525c56", "ff10498851524556",
		},
		tail3:  []string{"ff0287887f525c7f", "ff0287887f526e7f", "ff0287887f52787f"},
		finger: "c27141a82b314b670641a1d43c4ce62e683f8431d4d48fb2696b1f4b9be0bd61",
	},
	{
		name:    "btree21wd",
		order:   6,
		steps:   []int32{1555, 259, 43, 7, 1, 0},
		nSteps:  6,
		nParams: 139,
		nNodes:  9112,
		head10: []string{
			"0001616161616161", "00021c621c021c1c", "00031c621c021c1c", "00041c621c051c1c",
			"0005496251521456", "ff10496151525c56", "ff10496151524556", "ff10496151520d56",
			"ff10498851525c56", "ff10498851524556",
		},
		tail3:  []string{"ff0287887f525c7f", "ff0287887f526e7f", "ff0287887f52787f"},
		finger: "c8154f0d15a9f3bbbdbf44831315b78f38f465ba44dd21cb879fd154332b8168",
	},
}

func yyV2BTreeNodeHex(n uint64) string {
	s := strconv.FormatUint(n, 16)
	for len(s) < 16 {
		s = "0" + s
	}
	return s
}

// TestSeedV2BTreeDatSize .dat 体积必须远小于原始 1.1MB 的 mjs。
func TestSeedV2BTreeDatSize(t *testing.T) {
	st, err := os.Stat("seedv2_btree.dat")
	if err != nil {
		t.Fatalf("seedv2_btree.dat 不存在：%v", err)
	}
	if st.Size() >= 800*1024 {
		t.Fatalf(".dat 太大：%d 字节（要求 < 800KB）", st.Size())
	}
	if int64(len(yyV2BTreeData)) != st.Size() {
		t.Fatalf("go:embed 数据长度 %d 与文件大小 %d 不一致", len(yyV2BTreeData), st.Size())
	}
	t.Logf(".dat 大小 = %d 字节 (%.1f KB)", st.Size(), float64(st.Size())/1024)
}

// TestSeedV2BTreeLookup 五张表都能按名字取到，未知名字返回 nil。
func TestSeedV2BTreeLookup(t *testing.T) {
	for _, w := range yyV2BTreeWants {
		bt := yyV2BTreeByName(w.name)
		if bt == nil {
			t.Fatalf("yyV2BTreeByName(%q) = nil", w.name)
		}
		if bt.Name != w.name {
			t.Fatalf("%s: Name = %q", w.name, bt.Name)
		}
	}
	if bt := yyV2BTreeByName("btree99"); bt != nil {
		t.Fatalf("未知表名应返回 nil，实际 %+v", bt)
	}
	if bt := yyV2BTreeByName(""); bt != nil {
		t.Fatalf("空表名应返回 nil")
	}

	list := yyV2BTreeList()
	if len(list) != 5 {
		t.Fatalf("yyV2BTreeList() 长度 = %d，期望 5：%v", len(list), list)
	}
	for _, w := range yyV2BTreeWants {
		found := false
		for _, n := range list {
			if n == w.name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("yyV2BTreeList() 缺 %q：%v", w.name, list)
		}
	}
	t.Logf("已装载表 = %v", list)
}

// TestSeedV2BTreeShape 逐表断言 order / steps / param / nodes 数量。
func TestSeedV2BTreeShape(t *testing.T) {
	for _, w := range yyV2BTreeWants {
		bt := yyV2BTreeByName(w.name)
		if bt == nil {
			t.Fatalf("%s 缺失", w.name)
		}
		if bt.Order != w.order {
			t.Errorf("%s: Order = %d，期望 %d", w.name, bt.Order, w.order)
		}
		if len(bt.Steps) != w.nSteps {
			t.Errorf("%s: len(Steps) = %d，期望 %d", w.name, len(bt.Steps), w.nSteps)
		}
		if len(w.steps) != len(bt.Steps) {
			t.Errorf("%s: 期望 steps 长度 %d 与实际 %d 不符", w.name, len(w.steps), len(bt.Steps))
		} else {
			for i := range w.steps {
				if bt.Steps[i] != w.steps[i] {
					t.Errorf("%s: Steps[%d] = %d，期望 %d", w.name, i, bt.Steps[i], w.steps[i])
				}
			}
		}
		if len(bt.Param) != w.nParams {
			t.Errorf("%s: len(Param) = %d，期望 %d", w.name, len(bt.Param), w.nParams)
		}
		if len(bt.Nodes) != w.nNodes {
			t.Errorf("%s: len(Nodes) = %d，期望 %d", w.name, len(bt.Nodes), w.nNodes)
		}
	}
}

// TestSeedV2BTreeNodeBits 抽样前 10 / 后 3 个 node 的 uint64 逐位对拍。
// 这一条同时证明：小端写入、uint64 打包、高 16 位/低 48 位切分都没错。
func TestSeedV2BTreeNodeBits(t *testing.T) {
	for _, w := range yyV2BTreeWants {
		bt := yyV2BTreeByName(w.name)
		if bt == nil {
			t.Fatalf("%s 缺失", w.name)
		}
		if len(bt.Nodes) < 13 {
			t.Fatalf("%s: 节点太少 %d", w.name, len(bt.Nodes))
		}
		for i, want := range w.head10 {
			if got := yyV2BTreeNodeHex(bt.Nodes[i]); got != want {
				t.Errorf("%s: Nodes[%d] = %s，期望 %s", w.name, i, got, want)
			}
		}
		for i, want := range w.tail3 {
			idx := len(bt.Nodes) - len(w.tail3) + i
			if got := yyV2BTreeNodeHex(bt.Nodes[idx]); got != want {
				t.Errorf("%s: Nodes[%d] = %s，期望 %s", w.name, idx, got, want)
			}
		}
	}
}

// TestSeedV2BTreeFingerprint 全量数据指纹对拍（44k 个节点全覆盖，不只是抽样）。
func TestSeedV2BTreeFingerprint(t *testing.T) {
	for _, w := range yyV2BTreeWants {
		bt := yyV2BTreeByName(w.name)
		if bt == nil {
			t.Fatalf("%s 缺失", w.name)
		}
		var sb strings.Builder
		for i, n := range bt.Nodes {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(strconv.FormatUint(n, 16))
		}
		sb.WriteByte('|')
		for i, p := range bt.Param {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(strconv.FormatInt(int64(p[0]), 10))
			sb.WriteByte(':')
			sb.WriteString(strconv.FormatInt(int64(p[1]), 10))
		}
		sb.WriteByte('|')
		for i, s := range bt.Steps {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(strconv.FormatInt(int64(s), 10))
		}
		sum := sha256.Sum256([]byte(sb.String()))
		if got := hex.EncodeToString(sum[:]); got != w.finger {
			t.Errorf("%s: 指纹 = %s，期望 %s（%s 与 btree.mjs 不一致）", w.name, got, w.finger, w.name)
		}
	}
}

// TestSeedV2BTreeParamSanity param 区间成对有序、节点参数下标不越界。
func TestSeedV2BTreeParamSanity(t *testing.T) {
	for _, w := range yyV2BTreeWants {
		bt := yyV2BTreeByName(w.name)
		if bt == nil {
			t.Fatalf("%s 缺失", w.name)
		}
		for i, p := range bt.Param {
			if p[0] > p[1] {
				t.Fatalf("%s: Param[%d] = [%d,%d]，下界大于上界", w.name, i, p[0], p[1])
			}
		}
		for i, nd := range bt.Nodes {
			for b := 0; b < 6; b++ {
				pidx := int((nd >> (8 * uint(b))) & 0xFF)
				if pidx >= len(bt.Param) {
					t.Fatalf("%s: Nodes[%d] 第 %d 字节参数下标 %d 越界(param=%d)",
						w.name, i, b, pidx, len(bt.Param))
				}
			}
		}
	}
}

// TestSeedV2BTreeRootIsBranch 根节点的低 48 位必须是 6 个相同参数下标
// （cubiomes 的根是"全量区间"包络），高 16 位必须是第一层子节点起始下标 1
// —— node[0] 自己是包络，children 从 1 开始。对不上说明读错了字节偏移。
func TestSeedV2BTreeRootIsBranch(t *testing.T) {
	for _, w := range yyV2BTreeWants {
		bt := yyV2BTreeByName(w.name)
		if bt == nil {
			t.Fatalf("%s 缺失", w.name)
		}
		root := bt.Nodes[0]
		if hi := root >> 48; hi != 1 {
			t.Errorf("%s: 根节点高 16 位 = %#x，期望 1（第一层子节点起始下标）", w.name, hi)
		}
		pidx := byte(root & 0xFF)
		for b := 1; b < 6; b++ {
			if got := byte((root >> (8 * uint(b))) & 0xFF); got != pidx {
				t.Errorf("%s: 根节点第 %d 字节 = %d，期望与第 0 字节一致 %d",
					w.name, b, got, pidx)
			}
		}
		if int(pidx) >= len(bt.Param) {
			t.Errorf("%s: 根节点参数下标 %d 越界", w.name, pidx)
		}
	}
}
