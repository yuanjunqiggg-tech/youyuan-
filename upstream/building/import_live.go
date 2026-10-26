package building

import "time"

// Count accepted writes as they happen; world updates and checkpoint commits are separate.
func (t *ImportTask) trackSubmissions() func() {
	t.liveSubmitted, t.liveChunks = 0, 0
	t.liveColumns = make(map[[2]int]int)
	t.liveAt = time.Time{}
	set, fill, sub := t.SetBlock, t.FillRegion, t.OnSubProgress
	var lastSub time.Time
	if sub != nil {
		t.OnSubProgress = func(cx, cz, placed, total int) {
			if placed < total && !lastSub.IsZero() && time.Since(lastSub) < 100*time.Millisecond {
				return
			}
			lastSub = time.Now()
			sub(cx, cz, placed, total)
		}
	}
	if set != nil {
		t.SetBlock = func(x, y, z int32, name, states string) error {
			err := set(x, y, z, name, states)
			if err == nil && t.livePlacing {
				t.recordSubmitted(x, y, z, x, y, z)
			}
			return err
		}
	}
	if fill != nil {
		t.FillRegion = func(x1, y1, z1, x2, y2, z2 int32, name, states string) error {
			err := fill(x1, y1, z1, x2, y2, z2, name, states)
			if err == nil && t.livePlacing {
				t.recordSubmitted(x1, y1, z1, x2, y2, z2)
			}
			return err
		}
	}
	return func() {
		t.SetBlock, t.FillRegion, t.OnSubProgress = set, fill, sub
		t.livePlacing = false
		t.emitSubmitted(true)
	}
}

func (t *ImportTask) recordSubmitted(x1, y1, z1, x2, y2, z2 int32) {
	if t.OnSubmitted == nil {
		return
	}
	x1, x2 = x1-int32(t.BaseX), x2-int32(t.BaseX)
	z1, z2 = z1-int32(t.BaseZ), z2-int32(t.BaseZ)
	for cx := int(x1) / ChunkSize; cx <= int(x2)/ChunkSize; cx++ {
		for cz := int(z1) / ChunkSize; cz <= int(z2)/ChunkSize; cz++ {
			key := [2]int{cx, cz}
			before, total := t.liveColumns[key], t.chunkSolid[key]
			if total == 0 || before >= total {
				continue
			}
			width := min(int(x2), (cx+1)*ChunkSize-1) - max(int(x1), cx*ChunkSize) + 1
			depth := min(int(z2), (cz+1)*ChunkSize-1) - max(int(z1), cz*ChunkSize) + 1
			delta := min(width*depth*int(y2-y1+1), total-before)
			t.liveColumns[key] += delta
			t.liveSubmitted += delta
			if t.liveColumns[key] == total {
				t.liveChunks++
			}
		}
	}
	t.emitSubmitted(false)
}

func (t *ImportTask) emitSubmitted(force bool) {
	if t.OnSubmitted == nil {
		return
	}
	if !force && !t.liveAt.IsZero() && time.Since(t.liveAt) < 100*time.Millisecond {
		return
	}
	t.liveAt = time.Now()
	t.OnSubmitted(t.liveSubmitted, t.liveChunks)
}
