package building

import "testing"

// 验证实际导入引擎的完整遍历和 fill 合并；回调记录发出的指令体积，不伪造游戏联调。
func TestYouYuanImport4096Blocks(t *testing.T) {
	data := mkData(16)
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			for z := 0; z < 16; z++ {
				setBlockAt(t, data, x, y, z, "minecraft:stone", nil)
			}
		}
	}
	placed := int64(0)
	task := &ImportTask{Data: data, Speed: -1, StopCh: make(chan struct{}), SetBlock: func(x, y, z int32, name, states string) error { placed++; return nil }, FillRegion: func(x1, y1, z1, x2, y2, z2 int32, name, states string) error {
		placed += int64(x2-x1+1) * int64(y2-y1+1) * int64(z2-z1+1)
		return nil
	}}
	if err := task.Run(); err != nil {
		t.Fatal(err)
	}
	if placed != 4096 {
		t.Fatalf("超过 1000 方块时被截断：%d", placed)
	}
}
