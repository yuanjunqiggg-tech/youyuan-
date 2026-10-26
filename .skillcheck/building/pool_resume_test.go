package building

import "testing"

func TestPoolResumeSkipsOnlyCompletedUnits(t *testing.T) {
	res := &SplitResult{Units: []*Unit{{ID: 1, Kind: UnitNormal, BlockCount: 30}, {ID: 2, Kind: UnitNBT, BlockCount: 2}, {ID: 3, Kind: UnitSign, BlockCount: 1}}}
	p := NewWorkPool(res)
	c := p.Claim(UnitNormal, 0, 0)
	p.Complete(c.Item, c.Item.Unit.BlockCount)
	p.Complete(c.Item, c.Item.Unit.BlockCount)
	ids := p.CompletedUnits()
	if len(ids) != 1 || p.OverallProgress() != 30.0/33 {
		t.Fatal("double completion or wrong progress", ids, p.OverallProgress())
	}
	q := NewWorkPool(res)
	if e := q.RestoreCompleted(ids); e != nil {
		t.Fatal(e)
	}
	if c = q.Claim(UnitNormal, 0, 0); c.Item == nil || c.Item.Unit.ID != 2 {
		t.Fatal("completed normal work was repeated")
	}
	if e := q.RestoreCompleted([]int{999}); e == nil {
		t.Fatal("invalid layout accepted")
	}
}
