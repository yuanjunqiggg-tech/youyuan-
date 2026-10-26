package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

var yyTickingDimension = regexp.MustCompile(`%dimension\.dimensionName([012]):`)

// Parse the untranslated three-coordinate receipt as well as the older two-coordinate form.
func yyParseTickingAreas(resp *packet.CommandOutput) (map[string]*TickingAreaInfo, error) {
	if resp == nil {
		return nil, fmt.Errorf("常加载区域列表没有确认回执")
	}
	result := make(map[string]*TickingAreaInfo)
	dim := ""
	empty := false
	for _, msg := range resp.OutputMessages {
		// Mojang's all-dimensions empty-list reply has SuccessCount=0 despite being a valid result.
		if strings.TrimPrefix(msg.Message, "minecraft:") == "commands.tickingarea-list.failure.allDimensions" {
			empty = true
		}
		for _, text := range append([]string{msg.Message}, msg.Parameters...) {
			if strings.Contains(text, "commands.tickingarea-list.none") {
				empty = true
			}
			for _, line := range strings.Split(text, "\n") {
				if m := yyTickingDimension.FindStringSubmatch(line); m != nil {
					dim = m[1]
				}
				line = strings.TrimSpace(line)
				if !strings.HasPrefix(line, "- ") {
					continue
				}
				name, coords, ok := strings.Cut(strings.TrimPrefix(line, "- "), ": ")
				if !ok || name == "" || dim == "" {
					return nil, fmt.Errorf("常加载区域列表格式无法识别，不视为空列表")
				}
				var sx, sy, sz, ex, ey, ez float64
				var to string
				n, _ := fmt.Sscanf(coords, "%f %f %f %s %f %f %f", &sx, &sy, &sz, &to, &ex, &ey, &ez)
				if n != 7 || (to != "%commands.tickingarea-list.to" && to != "to") {
					n, _ = fmt.Sscanf(coords, "(%f, %f) to (%f, %f)", &sx, &sz, &ex, &ez)
					if n != 4 {
						return nil, fmt.Errorf("常加载区域 %s 坐标无法识别", name)
					}
				}
				if _, exists := result[name]; exists {
					return nil, fmt.Errorf("不同维度存在同名常加载区域 %s，不能按名称唯一识别", name)
				}
				result[name] = &TickingAreaInfo{Dimension: dim, StartX: sx, StartZ: sz, EndX: ex, EndZ: ez}
			}
		}
	}
	if len(result) == 0 && !empty {
		return nil, fmt.Errorf("常加载区域列表没有可识别的区域或空列表确认")
	}
	return result, nil
}
