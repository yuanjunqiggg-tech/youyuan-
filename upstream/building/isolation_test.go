package building

import "testing"

// TestNBTStructNameUniquePerBot §8.2：结构名必须跨机器人唯一、跨任务唯一。
func TestNBTStructNameUniquePerBot(t *testing.T) {
	name0 := NBTStructName(0, "chest", 1)
	name1 := NBTStructName(1, "chest", 1)
	if name0 == name1 {
		t.Fatalf("不同机器人的结构名不应相同：%q == %q", name0, name1)
	}
	// 同一机器人不同序号也不同（序号前缀保证确定性不同）
	if name0 == NBTStructName(0, "chest", 2) {
		t.Fatal("同机器人不同序号的结构名不应相同")
	}
	// 名字应包含机器人编号前缀
	if !hasSubstr(name0, "bot0") || !hasSubstr(name1, "bot1") {
		t.Errorf("结构名应含机器人编号前缀，got %q %q", name0, name1)
	}
}

// TestWorkspaceCenterNonOverlapping §8.3：不同机器人的工作台坐标互不重叠。
func TestWorkspaceCenterNonOverlapping(t *testing.T) {
	baseX, baseZ, sizeX, sizeZ := 100, 200, 50, 60
	seen := map[[3]int]bool{}
	for bot := 0; bot < 16; bot++ {
		x, y, z := WorkspaceCenter(bot, baseX, baseZ, sizeX, sizeZ)
		if y != WorkspaceY {
			t.Errorf("工作台 Y 应为 %d，got %d", WorkspaceY, y)
		}
		if seen[[3]int{x, y, z}] {
			t.Fatalf("机器人 %d 的工作台 (%d,%d,%d) 与前面的重叠", bot, x, y, z)
		}
		seen[[3]int{x, y, z}] = true
	}
	// 任意两机器人工作台间距应 ≥ 16（工作台占 ~11×11，不会重叠）
	for a := 0; a < 16; a++ {
		for b := a + 1; b < 16; b++ {
			ax, ay, az := WorkspaceCenter(a, baseX, baseZ, sizeX, sizeZ)
			bx, by, bz := WorkspaceCenter(b, baseX, baseZ, sizeX, sizeZ)
			dx := ax - bx
			if dx < 0 {
				dx = -dx
			}
			dz := az - bz
			if dz < 0 {
				dz = -dz
			}
			if dx < 16 && dz < 16 {
				t.Errorf("机器人 %d 与 %d 的工作台间距不足：dx=%d dz=%d", a, b, dx, dz)
			}
			_ = ay
			_ = by
		}
	}
}

func hasSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
