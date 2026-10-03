package main

import (
	"math"
	"testing"
	"time"
)

func TestNormalizeAngle(t *testing.T) {
	cases := []struct{ in, want float32 }{
		{0, 0}, {10, 10}, {-10, -10}, {350, -10}, {-350, 10},
		{180, 180}, {-180, 180}, {720, 0}, {-720, 0}, {360, 0},
	}
	for _, c := range cases {
		if got := normalizeAngle(c.in); got != c.want {
			t.Errorf("normalizeAngle(%v)=%v, want %v", c.in, got, c.want)
		}
	}
}

func TestClampPitch(t *testing.T) {
	if clampPitch(120) != 90 {
		t.Fatal("clampPitch(120) 应=90")
	}
	if clampPitch(-120) != -90 {
		t.Fatal("clampPitch(-120) 应=-90")
	}
	if clampPitch(30) != 30 {
		t.Fatal("clampPitch(30) 应=30")
	}
}

func TestSmoothRotateToInterpolates(t *testing.T) {
	bm := &BotManager{}
	bm.stopCh = make(chan struct{})
	bm.mu.Lock()
	bm.pitch = -10
	bm.yaw = 10
	bm.mu.Unlock()

	if err := bm.SmoothRotateTo(0, 0, 0.3); err != nil {
		t.Fatalf("SmoothRotateTo err: %v", err)
	}

	// ~100ms 时应在中途（-10→0 之间，未到目标）
	time.Sleep(100 * time.Millisecond)
	bm.mu.Lock()
	mid := bm.pitch
	bm.mu.Unlock()
	if mid >= -0.5 || mid <= -9.5 {
		t.Fatalf("100ms 应在插值中途, pitch=%v", mid)
	}

	// 等完成，应精确到 (0,0)
	time.Sleep(600 * time.Millisecond)
	bm.mu.Lock()
	fp, fy := bm.pitch, bm.yaw
	done := bm.rotStop == nil
	bm.mu.Unlock()
	if math.Abs(float64(fp)) > 1e-3 || math.Abs(float64(fy)) > 1e-3 {
		t.Fatalf("完成后应到(0,0), got (%v,%v)", fp, fy)
	}
	if !done {
		t.Fatal("完成后 rotStop 应被清空")
	}
}

// yaw 走最短路径：从 350 转到 10 只转 +20°（而非 -340° 长绕）。
// 最短路径中点应接近 0（350→0→10），长路径中点应接近 ±180。采样中点即可区分。
func TestSmoothRotateShortestYaw(t *testing.T) {
	bm := &BotManager{}
	bm.mu.Lock()
	bm.yaw = 350
	bm.mu.Unlock()
	bm.SmoothRotateTo(0, 10, 0.5) // 10 steps × 50ms
	time.Sleep(250 * time.Millisecond)
	bm.mu.Lock()
	mid := bm.yaw
	bm.mu.Unlock()
	if math.Abs(float64(mid)) > 60 {
		t.Fatalf("走了长路径(应走最短路径到10)，mid=%v", mid)
	}
}

func TestSetRotationCancelsSmooth(t *testing.T) {
	bm := &BotManager{}
	bm.mu.Lock()
	bm.pitch = 0
	bm.yaw = 0
	bm.mu.Unlock()

	bm.SmoothRotateTo(80, 80, 2) // 长平滑
	time.Sleep(100 * time.Millisecond)
	bm.SetRotation(5, 5) // 直接旋转应取消平滑
	time.Sleep(100 * time.Millisecond)

	bm.mu.Lock()
	p := bm.pitch
	bm.mu.Unlock()
	if p != 5 {
		t.Fatalf("SetRotation 应取消平滑并设为 5, got %v", p)
	}
	// 再等，确认不再向 80 移动
	time.Sleep(200 * time.Millisecond)
	bm.mu.Lock()
	p2 := bm.pitch
	bm.mu.Unlock()
	if p2 != 5 {
		t.Fatalf("平滑被取消后不应继续, got %v", p2)
	}
}
