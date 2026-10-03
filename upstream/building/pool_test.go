package building

import (
	"sync"
	"testing"
	"time"
)

// mkSplitResult 手工构造切分结果，便于精确控制单元构成。
func mkSplitResult(units ...*Unit) *SplitResult {
	res := &SplitResult{}
	total := 0
	for _, u := range units {
		res.Units = append(res.Units, u)
		total += u.BlockCount
	}
	res.TotalSolid = total
	return res
}

// runWorker 模拟一个工人工人池领取-执行-完成循环，直到所有单元完成。
// 阶段一（普通/NBT/轻NBT）全部共享，任意工人可领任意单元。
func runWorker(pool *WorkPool, seen *[]int, mu *sync.Mutex) {
	for {
		c := pool.Claim(UnitNormal, 0, 0)
		if c.Item == nil {
			if pool.Done() {
				return
			}
			time.Sleep(500 * time.Microsecond)
			continue
		}
		id := c.Item.Unit.ID
		mu.Lock()
		*seen = append(*seen, id)
		mu.Unlock()
		pool.Complete(c.Item, c.Item.Unit.BlockCount)
	}
}

// TestWorkStealingNoOverlap 多个工人并发领活：每个单元恰好被执行一次，无重叠，最终全部完成。
func TestWorkStealingNoOverlap(t *testing.T) {
	res := mkSplitResult(
		&Unit{ID: 0, Kind: UnitNormal, BlockCount: 10},
		&Unit{ID: 1, Kind: UnitNormal, BlockCount: 20},
		&Unit{ID: 2, Kind: UnitNormal, BlockCount: 5},
		&Unit{ID: 3, Kind: UnitNBT, BlockCount: 8},
		&Unit{ID: 4, Kind: UnitSign, BlockCount: 4},
	)
	pool := NewWorkPool(res)

	var mu sync.Mutex
	var seen []int
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runWorker(pool, &seen, &mu)
		}()
	}
	wg.Wait()

	if !pool.Done() {
		t.Fatal("所有单元应已完成")
	}
	if len(seen) != 5 {
		t.Fatalf("应执行 5 个单元，got %d（%v）", len(seen), seen)
	}
	uniq := map[int]bool{}
	for _, id := range seen {
		if uniq[id] {
			t.Fatalf("单元 %d 被执行多次（重叠）", id)
		}
		uniq[id] = true
	}
	for _, u := range res.Units {
		if !uniq[u.ID] {
			t.Errorf("单元 %d 从未被执行", u.ID)
		}
	}
}

// TestPhase2Gated 告示牌（阶段二）在阶段一未完成前不可被领。
func TestPhase2Gated(t *testing.T) {
	res := mkSplitResult(
		&Unit{ID: 0, Kind: UnitNormal, BlockCount: 5},
		&Unit{ID: 1, Kind: UnitSign, BlockCount: 2},
	)
	pool := NewWorkPool(res)

	// 第一次领 → 阶段一的普通单元
	c1 := pool.Claim(UnitNormal, 0, 0)
	if c1.Item == nil || c1.Item.Unit.Kind != UnitNormal {
		t.Fatal("第一次应领到普通单元")
	}
	// 普通还没完成 → 告示牌不可领
	if c := pool.Claim(UnitNormal, 0, 0); c.Item != nil {
		t.Fatal("阶段一未完成前不应领到告示牌（阶段二）")
	}
	// 完成普通 → 阶段二开放
	pool.Complete(c1.Item, c1.Item.Unit.BlockCount)
	c2 := pool.Claim(UnitNormal, 0, 0)
	if c2.Item == nil || c2.Item.Unit.Kind != UnitSign {
		t.Fatal("阶段一完成后应领到告示牌")
	}
}

// TestAnyBotCanClaimNBT 阶段一内任意机器人（无需工作台）都能直接领 NBT 单元（§7 援助 = 工作窃取）。
func TestAnyBotCanClaimNBT(t *testing.T) {
	res := mkSplitResult(
		&Unit{ID: 0, Kind: UnitNormal, BlockCount: 5},
		&Unit{ID: 1, Kind: UnitNBT, BlockCount: 8},
	)
	pool := NewWorkPool(res)

	// 任一工人可直接领到 NBT 单元（不做能力门槛）
	c := pool.Claim(UnitNormal, 0, 0)
	if c.Item == nil {
		t.Fatal("应能领到一个单元")
	}
	kind := c.Item.Unit.Kind
	if kind != UnitNormal && kind != UnitNBT {
		t.Fatalf("阶段一内应领到普通或NBT单元，got %v", kind)
	}
}

// TestProgressAggregation 进度按任务聚合，全部完成后各类进度均为 1。
func TestProgressAggregation(t *testing.T) {
	res := mkSplitResult(
		&Unit{ID: 0, Kind: UnitNormal, BlockCount: 10},
		&Unit{ID: 1, Kind: UnitNBT, BlockCount: 5},
		&Unit{ID: 2, Kind: UnitSign, BlockCount: 2},
	)
	pool := NewWorkPool(res)
	var fired int
	pool.OnProgress = func(normal, nbt, sign, light float64) {
		fired++
	}
	runWorker(pool, &[]int{}, &sync.Mutex{})

	n, nbt, s, l := pool.Progress()
	if n != 1 || nbt != 1 || s != 1 || l != 1 {
		t.Fatalf("完成后进度应为 (1,1,1,1)，got (%v,%v,%v,%v)", n, nbt, s, l)
	}
	if fired == 0 {
		t.Error("OnProgress 应被调用")
	}
}

// TestFailedUnitNotFakedDone 连续失败超限的单元应判为 failed（而非 done）：
// 方块未放置，进度到不了 100%，但池能收敛（Done 返回 true）不卡死；
// FailedBlocks 如实反映缺失量（§14.4 底线：不许"标记了却没放成"）。
func TestFailedUnitNotFakedDone(t *testing.T) {
	res := mkSplitResult(
		&Unit{ID: 0, Kind: UnitNormal, BlockCount: 100},
	)
	pool := NewWorkPool(res)

	c := pool.Claim(UnitNormal, 0, 0)
	if c.Item == nil {
		t.Fatal("应领到一个单元")
	}
	// 连续失败：MaxUnitFails=3，前两次 requeue，第三次判失败
	for i := 0; i < MaxUnitFails; i++ {
		if pool.Requeue(c.Item) && i == MaxUnitFails-1 {
			t.Fatal("最后一次 Requeue 应返回 false（判失败）")
		}
	}

	if got := pool.FailedBlocks(); got != 100 {
		t.Fatalf("FailedBlocks 应为 100（缺失量），got %d", got)
	}
	if got := pool.RemainingUnits(); got != 0 {
		t.Fatalf("判失败后不应再有未完成单元，got %d", got)
	}
	if !pool.Done() {
		t.Fatal("判失败后池应收敛（Done=true），不卡死")
	}
	n, _, _, _ := pool.Progress()
	if n == 1 {
		t.Fatal("判失败的单元未放置，进度不应到 100%")
	}
}

// TestReleasePending 断线回收：in_progress 单元 Release 后回到 pending，可被重新领到。
func TestReleasePending(t *testing.T) {
	res := mkSplitResult(&Unit{ID: 7, Kind: UnitNormal, BlockCount: 10})
	pool := NewWorkPool(res)

	c1 := pool.Claim(UnitNormal, 0, 0)
	if c1.Item == nil || c1.Item.Unit.ID != 7 {
		t.Fatal("应领到单元 7")
	}
	pool.Release(c1.Item)

	c2 := pool.Claim(UnitNormal, 0, 0)
	if c2.Item == nil || c2.Item.Unit.ID != 7 {
		t.Fatal("Release 后单元 7 应能被重新领到")
	}
}
