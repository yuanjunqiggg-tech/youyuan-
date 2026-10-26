package building

import "testing"

func TestShouldRepairBlockRect(t *testing.T) {
	task := &ImportTask{
		RepairMode: true,
		RepairRect: [6]int32{100, 64, 200, 150, 80, 250},
	}
	// Inside rect
	if !task.shouldRepairBlock(120, 70, 220) {
		t.Fatal("expected true for block inside rect")
	}
	// Outside rect (x too low)
	if task.shouldRepairBlock(99, 70, 220) {
		t.Fatal("expected false for x too low")
	}
	// Outside rect (y too high)
	if task.shouldRepairBlock(120, 81, 220) {
		t.Fatal("expected false for y too high")
	}
	// Boundary inclusive
	if !task.shouldRepairBlock(100, 64, 200) {
		t.Fatal("expected true at min boundary")
	}
	if !task.shouldRepairBlock(150, 80, 250) {
		t.Fatal("expected true at max boundary")
	}
}

func TestShouldRepairBlockCircle(t *testing.T) {
	task := &ImportTask{
		RepairMode:      true,
		RepairUseCircle: true,
		RepairCenterX:   0,
		RepairCenterZ:   0,
		RepairRadius:    20,
	}
	// Block in chunk 0,0 (0..15, 0..15) - chunk's closest point to center is (0,0), distance 0 <= 20
	if !task.shouldRepairBlock(5, 64, 5) {
		t.Fatal("expected true for block in chunk 0,0")
	}
	// Block in chunk 1,1 (16..31, 16..31) - closest point (16,16), distance sqrt(512)~22.6 > 20
	if task.shouldRepairBlock(20, 64, 20) {
		t.Fatal("expected false for block in chunk 1,1 (center 22.6 away)")
	}
	// Block in chunk (1,0) (16..31, 0..15) - closest point (16,0), distance 16 <= 20
	if !task.shouldRepairBlock(20, 64, 5) {
		t.Fatal("expected true for block in chunk 1,0")
	}
	// Non-repair mode always true
	normal := &ImportTask{}
	if !normal.shouldRepairBlock(0, 0, 0) {
		t.Fatal("expected true for non-repair mode")
	}
}
