package main

import (
	"fmt"
	"math"
	"strings"
)

// A lost mutation receipt is unknown; a confirmed read can resolve it without replaying the write.
func yyConfirmTickingChange(change func() (bool, error), verify func() (bool, error)) error {
	ok, err := change()
	if ok && err == nil {
		return nil
	}
	confirmed, readErr := verify()
	if confirmed && readErr == nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("常加载修改结果未确认（状态回读：%v）: %w", readErr, err)
	}
	if readErr != nil {
		return fmt.Errorf("常加载修改未确认，状态回读失败: %w", readErr)
	}
	return fmt.Errorf("服务器未达到所需的常加载区域状态")
}

func yyTickingDimensionKey(dim string) string {
	switch dim = strings.TrimPrefix(strings.ToLower(dim), "minecraft:"); dim {
	case "overworld", "0":
		return "0"
	case "nether", "the_nether", "1":
		return "1"
	case "end", "the_end", "2":
		return "2"
	}
	return dim
}

func yyTickingAreaCovers(a *TickingAreaInfo, dim string, x1, z1, x2, z2 int) bool {
	chunk := func(n float64) float64 { return math.Floor(n / 16) }
	return a != nil && yyTickingDimensionKey(a.Dimension) == yyTickingDimensionKey(dim) &&
		chunk(a.StartX) <= chunk(float64(x1)) && chunk(a.StartZ) <= chunk(float64(z1)) &&
		chunk(a.EndX) >= chunk(float64(x2)) && chunk(a.EndZ) >= chunk(float64(z2))
}
