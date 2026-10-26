package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/TriM-Organization/bedrock-world-operator/block"
	"github.com/TriM-Organization/bedrock-world-operator/chunk"
	"github.com/TriM-Organization/bedrock-world-operator/define"
)

// Read the server's real terrain stream when structure-template export is not
// available. Missing/undecodable subchunks are errors, never invented air.
func (bm *BotManager) yyReadRegionSubChunks(pos, size [3]int32) (map[string]any, error) {
	bm.mu.Lock()
	dim := bm.dimension
	bm.mu.Unlock()
	t := &SubChunkExportTask{bm: bm, minX: pos[0], minY: pos[1], minZ: pos[2], maxX: pos[0] + size[0] - 1, maxY: pos[1] + size[1] - 1, maxZ: pos[2] + size[2] - 1, dimension: dim}
	columns := map[[2]int32][]*chunk.SubChunk{}
	var entities []map[string]any
	for x := t.minX >> 4; x <= t.maxX>>4; x++ {
		for z := t.minZ >> 4; z <= t.maxZ>>4; z++ {
			subs, nbtData, e := t.fetchSubChunks(x, z, define.Dimension(dim).Range())
			if e != nil {
				return nil, e
			}
			columns[[2]int32{x, z}] = subs
			entities = append(entities, nbtData...)
		}
	}
	return yySubChunksToStructure(pos, size, columns, entities)
}

func yySubChunksToStructure(pos, size [3]int32, columns map[[2]int32][]*chunk.SubChunk, entities []map[string]any) (map[string]any, error) {
	if e := yyValidateRegion(pos, size); e != nil {
		return nil, e
	}
	volume := int64(1)
	for _, s := range size {
		if int64(s) > int64(^uint(0)>>1)/8/volume {
			return nil, fmt.Errorf("区域数组超出平台可分配范围")
		}
		volume *= int64(s)
	}
	indices := []any{make([]int32, int(volume)), make([]int32, int(volume))}
	palette := []any{}
	paletteIDs := map[string]int32{}
	for layer := 0; layer < 2; layer++ {
		out := indices[layer].([]int32)
		for x := int32(0); x < size[0]; x++ {
			for y := int32(0); y < size[1]; y++ {
				for z := int32(0); z < size[2]; z++ {
					wx, wy, wz := pos[0]+x, pos[1]+y, pos[2]+z
					column, exists := columns[[2]int32{wx >> 4, wz >> 4}]
					si := (wy >> 4) - (pos[1] >> 4)
					if !exists || si < 0 || int(si) >= len(column) {
						return nil, fmt.Errorf("区域缺失子区块 (%d,%d,%d)", wx>>4, wy>>4, wz>>4)
					}
					idx := int((int64(x)*int64(size[1])+int64(y))*int64(size[2]) + int64(z))
					name, states := "minecraft:air", map[string]any{}
					if sub := column[si]; sub != nil {
						if layer > 0 && len(sub.Layers()) <= layer {
							out[idx] = -1
							continue
						}
						var ok bool
						rid := sub.Block(byte(wx&15), byte(wy&15), byte(wz&15), uint8(layer))
						name, states, ok = block.RuntimeIDToState(rid)
						if !ok {
							return nil, fmt.Errorf("当前调色板不能识别服务器方块 runtime_id=%d", rid)
						}
					} else if layer > 0 {
						out[idx] = -1
						continue
					}
					if layer > 0 && name == "minecraft:air" {
						out[idx] = -1
						continue
					}
					keyBytes, _ := json.Marshal([]any{name, states})
					key := string(keyBytes)
					pi, ok := paletteIDs[key]
					if !ok {
						pi = int32(len(palette))
						paletteIDs[key] = pi
						palette = append(palette, map[string]any{"name": name, "states": states, "version": int32(18168865)})
					}
					out[idx] = pi
				}
			}
		}
	}
	bpd := map[string]any{}
	for _, entity := range entities {
		x, xOK := numToInt32(entity["x"])
		y, yOK := numToInt32(entity["y"])
		z, zOK := numToInt32(entity["z"])
		if !xOK || !yOK || !zOK {
			return nil, fmt.Errorf("服务器方块实体缺少有效坐标")
		}
		if x < pos[0] || y < pos[1] || z < pos[2] || x >= pos[0]+size[0] || y >= pos[1]+size[1] || z >= pos[2]+size[2] {
			continue
		}
		i := ((int64(x)-int64(pos[0]))*int64(size[1])+int64(y)-int64(pos[1]))*int64(size[2]) + int64(z) - int64(pos[2])
		bpd[strconv.Itoa(int(i))] = map[string]any{"block_entity_data": entity}
	}
	return map[string]any{"format_version": int32(1), "size": []int32{size[0], size[1], size[2]}, "structure_world_origin": []int32{pos[0], pos[1], pos[2]}, "structure": map[string]any{"block_indices": indices, "entities": []any{}, "palette": map[string]any{"default": map[string]any{"block_palette": palette, "block_position_data": bpd}}}, "youyuan_read_source": "SubChunk", "youyuan_entities_included": false}, nil
}

// Request-region size uses wire int32s; avoid overflow before allocation.
func yyValidateRegion(pos, size [3]int32) error {
	for i, s := range size {
		if s <= 0 || int64(pos[i])+int64(s)-1 > 2147483647 {
			return fmt.Errorf("无效区域坐标或尺寸")
		}
	}
	return nil
}
