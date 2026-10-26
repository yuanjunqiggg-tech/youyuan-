package building

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	_ "embed"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
)

//go:embed bdx_runtime_ids.json
var bdxRuntimeIDsJSON []byte

// runtimeIDs: runtime_id → [block_name, data_value]
var bdxRuntimeIDs [][2]any

func init() {
	json.Unmarshal(bdxRuntimeIDsJSON, &bdxRuntimeIDs)
}

// BDX operation codes (from BDXConverter / TriM-Organization)
const (
	bdxCreateConstantString             = 0x01
	bdxPlaceBlockWithBlockStates        = 0x05
	bdxAddInt16ZValue0                  = 0x06
	bdxPlaceBlock                       = 0x07
	bdxAddZValue0                       = 0x08
	bdxNOP                              = 0x09
	bdxAddInt32ZValue0                  = 0x0c
	bdxPlaceBlockWithBlockStatesDepr    = 0x0d // deprecated variant
	bdxAddXValue                        = 0x0e
	bdxSubtractXValue                   = 0x0f
	bdxAddYValue                        = 0x10
	bdxSubtractYValue                   = 0x11
	bdxAddZValue                        = 0x12
	bdxSubtractZValue                   = 0x13
	bdxAddInt16XValue                   = 0x14
	bdxAddInt32XValue                   = 0x15
	bdxAddInt16YValue                   = 0x16
	bdxAddInt32YValue                   = 0x17
	bdxAddInt16ZValue                   = 0x18
	bdxAddInt32ZValue                   = 0x19
	bdxSetCommandBlockData              = 0x1a
	bdxPlaceBlockWithCommandBlockData   = 0x1b
	bdxAddInt8XValue                    = 0x1c
	bdxAddInt8YValue                    = 0x1d
	bdxAddInt8ZValue                    = 0x1e
	bdxUseRuntimeIDPool                 = 0x1f
	bdxPlaceRuntimeBlock                = 0x20
	bdxPlaceBlockWithRuntimeId          = 0x21
	bdxPlaceRuntimeBlockWithCmdBlock    = 0x22
	bdxPlaceRuntimeBlockWithCmdBlockU32 = 0x23
	bdxPlaceCommandBlockWithCmdBlockData = 0x24
	bdxPlaceRuntimeBlockWithChestData   = 0x25
	bdxPlaceRuntimeBlockWithChestDataU32 = 0x26
	bdxAssignDebugData                  = 0x27
	bdxPlaceBlockWithChestData          = 0x28
	bdxPlaceBlockWithNBTData            = 0x29
	bdxTerminate                        = 0x58 // 'X' = 88 in BDXConverter, but some use 0x58
)

func brotliDecode(data []byte) ([]byte, error) {
	r := brotli.NewReader(bytes.NewReader(data))
	var out bytes.Buffer
	_, err := out.ReadFrom(r)
	return out.Bytes(), err
}

// ParseBDX parses a BDX file. Handles brotli-compressed ("BD@") format (BDXConverter standard).
func ParseBDX(data []byte) (*StructureData, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("ParseBDX: file too short (%d bytes)", len(data))
	}

	// BDXConverter format: "BD@" magic + brotli-compressed inner data
	// BD@ variant uses uint16BE for string pool IDs (not varuint).
	isBDXConverter := false
	if data[0] == 'B' && data[1] == 'D' && data[2] == '@' {
		decompressed, err := brotliDecode(data[3:])
		if err != nil {
			return nil, fmt.Errorf("ParseBDX: brotli decompress: %w", err)
		}
		data = decompressed
		isBDXConverter = true
	}

	// Inner format: "BDX\x00" + author (C string) + operations
	if len(data) < 4 || data[0] != 'B' || data[1] != 'D' || data[2] != 'X' || data[3] != 0x00 {
		// Try legacy "BD" header
		if len(data) >= 3 && data[0] == 'B' && data[1] == 'D' {
			return nil, fmt.Errorf("ParseBDX: legacy BD format not supported, use BDXConverter format")
		}
		return nil, fmt.Errorf("ParseBDX: invalid inner header: %q", data[:min(4, len(data))])
	}
	pos := 4 // skip "BDX\x00"

	// Skip author (C string)
	pos = skipCString(data, pos)
	if pos < 0 {
		return nil, fmt.Errorf("ParseBDX: truncated author string")
	}

	stringPool := []string{}
	runtimeIDPool := []int{}
	var pendingCmdBlock *map[string]any // SetCommandBlockData stores data for next placement
	var posX, posY, posZ int32
	var minX, maxX, minY, maxY, minZ, maxZ int32

	type blockOp struct {
		x, y, z   int32
		name      string
		states    string
		dataVal   uint32
		runtimeID int
		isRuntime bool // 该方块由 runtime ID 放置（朝向等状态编码在 data value 里）
		cmdData   map[string]any
		nbtData   map[string]any
	}
	var blocks []blockOp

	r := &bdxReader{buf: data, pos: pos, useBE: isBDXConverter}

loop:
	for {
		if r.pos >= len(r.buf) {
			break
		}
		opCode := r.buf[r.pos]
		r.pos++

		if opCode == bdxTerminate {
			break // 0x58 ('X') 终止符
		}

		switch opCode {
		case bdxCreateConstantString:
			s := r.readCString()
			stringPool = append(stringPool, s)

		case bdxPlaceBlockWithBlockStates:
			bid := r.readID()
			sid := r.readID()
			name := getStr(stringPool, bid)
			states := getStr(stringPool, sid)
			blocks = append(blocks, blockOp{x: posX, y: posY, z: posZ, name: name, states: states})

		case bdxPlaceBlockWithBlockStatesDepr:
			// Deprecated format: blockConstantStringID(uint16) + blockStatesString(null-terminated C string)
			bid := r.readID()
			states := r.readCString()
			name := getStr(stringPool, bid)
			blocks = append(blocks, blockOp{x: posX, y: posY, z: posZ, name: name, states: states})

		case bdxPlaceBlock:
			bid := r.readID()
			dv := r.readID()
			name := getStr(stringPool, bid)
			blocks = append(blocks, blockOp{x: posX, y: posY, z: posZ, name: name, dataVal: dv})

		case bdxPlaceBlockWithCommandBlockData:
			bid := r.readID()
			dv := r.readID()
			cd := r.readCmdBlock()
			name := getStr(stringPool, bid)
			blocks = append(blocks, blockOp{x: posX, y: posY, z: posZ, name: name, dataVal: dv, cmdData: cd})

		case bdxAddInt16ZValue0:
			posZ += int32(r.readInt16BE())
		case bdxAddZValue0:
			posZ++
		case bdxNOP, 0x00: // NOP, or padding byte in BD@ format
		case bdxAddInt32ZValue0:
			posZ += r.readInt32BE()

		case bdxAddXValue:
			posX++
		case bdxSubtractXValue:
			posX--
		case bdxAddYValue:
			posY++
		case bdxSubtractYValue:
			posY--
		case bdxAddZValue:
			posZ++
		case bdxSubtractZValue:
			posZ--

		case bdxAddInt16XValue:
			posX += int32(r.readInt16BE())
		case bdxAddInt32XValue:
			posX += r.readInt32BE()
		case bdxAddInt16YValue:
			posY += int32(r.readInt16BE())
		case bdxAddInt32YValue:
			posY += r.readInt32BE()
		case bdxAddInt16ZValue:
			posZ += int32(r.readInt16BE())
		case bdxAddInt32ZValue:
			posZ += r.readInt32BE()

		case bdxAddInt8XValue:
			posX += int32(int8(r.mustByte()))
		case bdxAddInt8YValue:
			posY += int32(int8(r.mustByte()))
		case bdxAddInt8ZValue:
			posZ += int32(int8(r.mustByte()))

		case bdxUseRuntimeIDPool:
			count := r.readID()
			for i := uint32(0); i < count; i++ {
				runtimeIDPool = append(runtimeIDPool, int(r.readID()))
			}

		case bdxPlaceRuntimeBlock:
			rtid := r.readID()
			blocks = append(blocks, blockOp{x: posX, y: posY, z: posZ, runtimeID: int(rtid), isRuntime: true})

		case bdxPlaceBlockWithRuntimeId:
			bid := r.readID()
			rtid := r.readUint32BE()
			name := getStr(stringPool, bid)
			blocks = append(blocks, blockOp{x: posX, y: posY, z: posZ, name: name, runtimeID: int(rtid), isRuntime: true})

		case bdxSetCommandBlockData:
			cd := r.readCmdBlock()
			pendingCmdBlock = &cd

		case bdxPlaceRuntimeBlockWithCmdBlock, bdxPlaceRuntimeBlockWithCmdBlockU32:
			if opCode == bdxPlaceRuntimeBlockWithCmdBlockU32 {
				r.readUint32LE()
			}
			rtid := r.readID()
			var cd map[string]any
			if r.useBE {
				cd = r.readBDXNBTCompound()
			} else {
				cd = r.readCmdBlock()
			}
			blocks = append(blocks, blockOp{x: posX, y: posY, z: posZ, runtimeID: int(rtid), cmdData: cd, isRuntime: true})

		case bdxPlaceCommandBlockWithCmdBlockData:
			bid := r.readID()
			name := getStr(stringPool, bid)
			var cd map[string]any
			if r.useBE {
				cd = r.readBDXNBTCompound()
			} else {
				cd = r.readCmdBlock()
			}
			blocks = append(blocks, blockOp{x: posX, y: posY, z: posZ, name: name, cmdData: cd})

		case bdxPlaceRuntimeBlockWithChestData, bdxPlaceRuntimeBlockWithChestDataU32:
			if opCode == bdxPlaceRuntimeBlockWithChestDataU32 {
				r.readUint32LE()
			}
			rtid := r.readID()
			nbt := r.readChestData()
			blocks = append(blocks, blockOp{x: posX, y: posY, z: posZ, runtimeID: int(rtid), nbtData: nbt, isRuntime: true})

		case bdxPlaceBlockWithChestData:
			bid := r.readID()
			dv := r.readID()
			nbt := r.readChestData()
			name := getStr(stringPool, bid)
			blocks = append(blocks, blockOp{x: posX, y: posY, z: posZ, name: name, dataVal: dv, nbtData: nbt})

		case bdxPlaceBlockWithNBTData:
			bid := r.readID()
			sid := r.readID()
			if r.useBE {
				r.readID() // BDXConverter writes blockStatesConstantStringID twice
			}
			nbt := r.readBDXNBTCompound()
			name := getStr(stringPool, bid)
			states := getStr(stringPool, sid)
			blocks = append(blocks, blockOp{x: posX, y: posY, z: posZ, name: name, states: states, nbtData: nbt})

		case bdxAssignDebugData:
			_ = r.readCString() // debug data

		default:
			// Unknown opcode — stop gracefully in BE mode
			if r.useBE {
				break loop
			}
			return nil, fmt.Errorf("ParseBDX: unknown opcode 0x%02x at offset %d", opCode, r.pos-1)
		}

		// Apply pending command block data to the last placed block
		if pendingCmdBlock != nil && len(blocks) > 0 {
			last := &blocks[len(blocks)-1]
			if last.cmdData == nil {
				last.cmdData = *pendingCmdBlock
			}
			pendingCmdBlock = nil
		}
	}

	// Calculate bounds from actual block placements
	for _, op := range blocks {
		if op.x < minX { minX = op.x }
		if op.x > maxX { maxX = op.x }
		if op.y < minY { minY = op.y }
		if op.y > maxY { maxY = op.y }
		if op.z < minZ { minZ = op.z }
		if op.z > maxZ { maxZ = op.z }
	}
	sx := int(maxX - minX + 1)
	sy := int(maxY - minY + 1)
	sz := int(maxZ - minZ + 1)
	if sx <= 0 { sx = 1 }
	if sy <= 0 { sy = 1 }
	if sz <= 0 { sz = 1 }

	result := &StructureData{
		SizeX:  sx,
		SizeY:  sy,
		SizeZ:  sz,
		Blocks: make(map[uint64]uint32),
	}

	for _, op := range blocks {
		bx := int(op.x - minX)
		by := int(op.y - minY)
		bz := int(op.z - minZ)
		if bx < 0 || bx >= sx || by < 0 || by >= sy || bz < 0 || bz >= sz {
			continue
		}

		name := op.name
		if name == "" {
			// 二级查找：先查运行时 ID pool，再查 bdxRuntimeIDs
			rid := op.runtimeID
			if rid >= 0 && rid < len(runtimeIDPool) {
				rid = runtimeIDPool[rid]
			}
			if rid >= 0 && rid < len(bdxRuntimeIDs) {
				pair := bdxRuntimeIDs[rid]
				if len(pair) >= 1 {
					if s, ok := pair[0].(string); ok {
						name = s
					}
				}
			}
		}
		if name == "" && op.runtimeID >= 0 {
			name = fmt.Sprintf("unknown_rid_%d", op.runtimeID)
		}
		if name == "air" || name == "minecraft:air" || name == "" {
			continue
		}
		// 修复: runtime ID 放置的方块，朝向等状态编码在 data value 里。
		// 此前只取方块名、丢弃 data value，导致命令方块等默认朝下。这里恢复 data→states。
		if op.isRuntime && op.states == "" {
			if st, ok := runtimeBlockStates(op.runtimeID, runtimeIDPool, name); ok {
				op.states = st
			}
		}
		if !strings.HasPrefix(name, "minecraft:") {
			name = "minecraft:" + name
		}

		states := op.states
		if states == "" && op.dataVal != 0 {
			states = fmt.Sprintf("%d", op.dataVal)
		}
		if states == "" {
			states = "[]"
		}

		pos := packPos(int32(bx), int32(by), int32(bz))
		idx := result.getOrCreatePaletteIndex(name, states)
		result.Blocks[pos] = idx

		nbt := op.cmdData
		if nbt == nil {
			nbt = op.nbtData
		}
		if nbt != nil && len(nbt) > 0 {
			result.ensureNBTBlocks()
			result.NBTBlocks[pos] = nbt
		}
	}
	blocks = nil

	return result, nil
}

// ── runtime ID → 方块状态恢复 ──

var (
	dataStatesOnce sync.Once
	dataStates     map[string]string // "name\x00data" → states 字符串
)

// buildDataStates 从 block_ids.json（bedrockBlockMap）构建 name+data → states 反查表。
// 旧版方块 data value 编码了朝向/条件位等状态，与 bdx_runtime_ids 的 data value 一致。
func buildDataStates() {
	dataStates = make(map[string]string)
	for _, dataMap := range bedrockBlockMap {
		for data, val := range dataMap {
			name, states := splitNameStates(val)
			if name == "" || states == "[]" {
				continue
			}
			dataStates[name+"\x00"+data] = states
		}
	}
}

// lookupDataStates 把某方块的 legacy data value 转成 states 字符串。
func lookupDataStates(name string, data uint32) (string, bool) {
	dataStatesOnce.Do(buildDataStates)
	s, ok := dataStates[name+"\x00"+strconv.FormatUint(uint64(data), 10)]
	return s, ok
}

// runtimeBlockStates 从 runtime ID（经 pool 解析）的 data value 恢复方块状态。
func runtimeBlockStates(rid int, pool []int, name string) (string, bool) {
	if rid >= 0 && rid < len(pool) {
		rid = pool[rid]
	}
	if rid < 0 || rid >= len(bdxRuntimeIDs) {
		return "", false
	}
	pair := bdxRuntimeIDs[rid]
	if len(pair) < 2 {
		return "", false
	}
	d, ok := pair[1].(float64)
	if !ok {
		return "", false
	}
	return lookupDataStates(name, uint32(d))
}

// ── bdxReader ──

type bdxReader struct {
	buf   []byte
	pos   int
	useBE bool // BDXConverter variant uses uint16BE for pool IDs
}

func (r *bdxReader) readID() uint32 {
	if r.useBE {
		return uint32(r.readUint16BE())
	}
	return r.readVarUint()
}

func (r *bdxReader) mustByte() byte {
	if r.pos >= len(r.buf) {
		return 0
	}
	b := r.buf[r.pos]
	r.pos++
	return b
}

func (r *bdxReader) readVarUint() uint32 {
	var result uint32
	var shift uint
	for r.pos < len(r.buf) {
		b := r.buf[r.pos]
		r.pos++
		result |= uint32(b&0x7F) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
	}
	return result
}

func (r *bdxReader) readUint32LE() uint32 {
	if r.pos+4 > len(r.buf) {
		return 0
	}
	v := binary.LittleEndian.Uint32(r.buf[r.pos:])
	r.pos += 4
	return v
}

func (r *bdxReader) readUint16BE() uint16 {
	if r.pos+2 > len(r.buf) {
		return 0
	}
	v := binary.BigEndian.Uint16(r.buf[r.pos:])
	r.pos += 2
	return v
}

func (r *bdxReader) readInt16BE() int16 {
	return int16(r.readUint16BE())
}

func (r *bdxReader) readInt32BE() int32 {
	if r.pos+4 > len(r.buf) {
		return 0
	}
	v := int32(binary.BigEndian.Uint32(r.buf[r.pos:]))
	r.pos += 4
	return v
}

func (r *bdxReader) readUint32BE() uint32 {
	if r.pos+4 > len(r.buf) {
		return 0
	}
	v := binary.BigEndian.Uint32(r.buf[r.pos:])
	r.pos += 4
	return v
}

func (r *bdxReader) readCString() string {
	end := r.pos
	for end < len(r.buf) && r.buf[end] != 0x00 {
		end++
	}
	s := string(r.buf[r.pos:end])
	r.pos = end + 1
	return s
}

// readCmdBlock reads command block data (BDXConverter format).
// Format: mode(uint32 BE) + command(C str) + customName(C str) + lastOutput(C str)
//         + tickDelay(int32 BE, signed) + executeOnFirstTick(1 byte) + trackOutput(1 byte)
//         + conditional(1 byte) + needsRedstone(1 byte)
func (r *bdxReader) readCmdBlock() map[string]any {
	var mode uint32
	var tickDelay int32
	if r.useBE {
		mode = r.readUint32BE()
	} else {
		mode = r.readVarUint()
	}
	cmd := r.readCString()
	customName := r.readCString()
	lastOutput := r.readCString()
	if r.useBE {
		tickDelay = r.readInt32BE()
	} else {
		tickDelay = int32(r.readUint32LE())
	}
	exeFirst := r.mustByte()
	trackOut := r.mustByte()
	cond := r.mustByte()
	needsRS := r.mustByte()
	return map[string]any{
		"Mode":              mode,
		"Command":           cmd,
		"CustomName":        customName,
		"LastOutput":        lastOutput,
		"TickDelay":         tickDelay,
		"ExecuteOnFirstTick": exeFirst != 0,
		"TrackOutput":       trackOut != 0,
		"Conditional":       cond != 0,
		"NeedsRedstone":     needsRS != 0,
	}
}

// readChestData 解析 BDX 格式的容器物品数据，返回 {"Items": [...]} NBT。
// 每个槽位: itemName(C串) + count(1B) + data(2B) + slotID(1B)
// 终止条件: 下一字节为 0x00 或 >=0x80
func (r *bdxReader) readChestData() map[string]any {
	var items []any
	if r.useBE {
		// BDXConverter format: slotCount(1B) then exactly N slots
		slotCount := int(r.mustByte())
		for i := 0; i < slotCount; i++ {
			itemName := r.readCString()
			if itemName == "" { break }
			count := int(r.mustByte())
			r.pos += 2 // data value (2 bytes, unused)
			slotID := int(r.mustByte())
			items = append(items, map[string]any{
				"Name": "minecraft:" + itemName, "Count": int32(count), "Slot": int32(slotID),
			})
		}
	} else {
		// Legacy format: terminate on 0x00 or >=0x80
		for {
			if r.pos >= len(r.buf) { break }
			itemName := r.readCString()
			if itemName == "" || r.pos >= len(r.buf) { break }
			count := int(r.mustByte())
			r.pos += 2
			slotID := int(r.mustByte())
			items = append(items, map[string]any{
				"Name": "minecraft:" + itemName, "Count": int32(count), "Slot": int32(slotID),
			})
			if r.pos >= len(r.buf) || r.buf[r.pos] == 0x00 || r.buf[r.pos] >= 0x80 { break }
		}
	}
	if len(items) == 0 { return nil }
	return map[string]any{"Items": items}
}

// readBDXNBTCompound 读取 BDX 格式的完整 NBT 复合标签。
// BDX NBT 格式为标准 Bedrock Little-Endian NBT（与 MCWorld LevelDB 格式兼容）。
func (r *bdxReader) readBDXNBTCompound() map[string]any {
	// BDX NBT: tag_type(1B) + name_len(uint16 LE) + name + value
	// For compounds: 0x0a + uint16LE name_len + name + compound_body
	reader := bytes.NewReader(r.buf[r.pos:])
	tagType, _ := reader.ReadByte()
	if tagType != 0x0a {
		// Not a compound — might be empty or a different type
		// Advance by 1 (the tag type byte we just read)
		r.pos++
		return nil
	}
	var nameLen [2]byte
	io.ReadFull(reader, nameLen[:])
	nl := int(binary.LittleEndian.Uint16(nameLen[:]))
	reader.Seek(int64(nl), io.SeekCurrent)

	compound, err := parseNBTCompound(reader)
	if err != nil {
		// Don't update r.pos on error -- the parse may have consumed
		// a partial amount, and the caller should not skip past it.
		return nil
	}
	r.pos = len(r.buf) - reader.Len()
	return compound
}

func skipCString(data []byte, pos int) int {
	for pos < len(data) && data[pos] != 0x00 {
		pos++
	}
	if pos >= len(data) {
		return -1
	}
	return pos + 1
}

func getStr(pool []string, idx uint32) string {
	if int(idx) < len(pool) {
		return pool[idx]
	}
	return ""
}
