package building

import "fmt"

const MaxTickingAreaChunks = 100
const MaxWorldTickingAreas = 10
const MaxImportRegionChunks = 10

func ImportRegionSize(mode, size int) int {
	return (&ImportTask{RegionMode: mode, RegionSizeChunks: size}).regionChunks()
}

func ValidateImportPreload(size, batch, x, z int, enabled bool) error {
	if size == 0 {
		return nil
	}
	if batch < 1 {
		return fmt.Errorf("导入批次边长无效")
	}
	if !enabled {
		return fmt.Errorf("preload_size_chunks 需要启用 ticking_preload")
	}
	if err := ValidateImportRegion(size, x, z, true); err != nil {
		return err
	}
	if size < batch || size%batch != 0 {
		return fmt.Errorf("常加载窗口边长必须不小于导入批次且为其整数倍")
	}
	return nil
}

func chunkFloor(v int) int {
	q := v / ChunkSize
	if v%ChunkSize < 0 {
		q--
	}
	return q
}

// ValidateTickingArea counts world chunks, including partial boundary chunks.
func ValidateTickingArea(x1, z1, x2, z2 int) error {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if z1 > z2 {
		z1, z2 = z2, z1
	}
	nx := int64(chunkFloor(x2)) - int64(chunkFloor(x1)) + 1
	nz := int64(chunkFloor(z2)) - int64(chunkFloor(z1)) + 1
	if nx > MaxTickingAreaChunks || nz > MaxTickingAreaChunks || nx*nz > MaxTickingAreaChunks {
		return fmt.Errorf("常加载区域覆盖 %d×%d 个世界区块，单个区域最多 %d 个", nx, nz, MaxTickingAreaChunks)
	}
	return nil
}

func ValidateImportRegion(size, x, z int, preload bool) error {
	if size < 0 || size > MaxImportRegionChunks {
		return fmt.Errorf("region_size_chunks 必须为 0（沿用旧档位）或 1..10")
	}
	if size > 5 && !preload {
		return fmt.Errorf("超过 5×5 的导入区域需要启用 ticking_preload")
	}
	if size > 0 {
		// Check the worst full batch without adding to untrusted coordinates.
		rx, rz := x%16, z%16
		if rx < 0 {
			rx += 16
		}
		if rz < 0 {
			rz += 16
		}
		return ValidateTickingArea(rx, rz, rx+size*16-1, rz+size*16-1)
	}
	return nil
}
