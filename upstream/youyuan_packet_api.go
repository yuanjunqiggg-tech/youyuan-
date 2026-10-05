package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	lua "github.com/yuin/gopher-lua"
)

var yyPacketSequence atomic.Uint64
var yyPacketIDsOnce sync.Once
var yyPacketIDs map[string]uint32

// Map snake_case fields back to the actual protocol types. Values are validated
// before touching the connection; invalid/unsupported unions are never zeroed.
func yyFillPacketValue(dst reflect.Value, input any, path string) error {
	if !dst.CanSet() {
		return fmt.Errorf("%s 不可写", path)
	}
	if input == nil {
		dst.SetZero()
		return nil
	}
	if dst.Type() == reflect.TypeOf(protocol.Bitset{}) {
		values, ok := input.([]any)
		if !ok {
			return fmt.Errorf("%s 需要置位索引数组", path)
		}
		bits := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
		for _, v := range values {
			n, e := strconv.Atoi(fmt.Sprint(v))
			if e != nil || n < 0 || n >= bits.Len() {
				return fmt.Errorf("%s 无效输入标志", path)
			}
			bits.Set(n)
		}
		dst.Set(reflect.ValueOf(bits))
		return nil
	}
	switch dst.Kind() {
	case reflect.Ptr:
		dst.Set(reflect.New(dst.Type().Elem()))
		return yyFillPacketValue(dst.Elem(), input, path)
	case reflect.Interface:
		if dst.NumMethod() == 0 {
			dst.Set(reflect.ValueOf(input))
			return nil
		}
		return fmt.Errorf("%s 是协议联合类型 %s；请用高级动作API或payload_hex提供完整包体", path, dst.Type())
	case reflect.Struct:
		fields, ok := input.(map[string]any)
		if !ok {
			return fmt.Errorf("%s 需要字段对象", path)
		}
		indices := map[string]int{}
		for i := 0; i < dst.NumField(); i++ {
			f := dst.Type().Field(i)
			if f.IsExported() {
				indices[snakeCase(f.Name)] = i
			}
		}
		for name, v := range fields {
			i, ok := indices[name]
			if !ok {
				return fmt.Errorf("%s 未知字段 %s", path, name)
			}
			if e := yyFillPacketValue(dst.Field(i), v, path+"."+name); e != nil {
				return e
			}
		}
		return nil
	case reflect.Slice, reflect.Array:
		if dst.Type().Elem().Kind() == reflect.Uint8 {
			if str, ok := input.(string); ok {
				b, e := hex.DecodeString(strings.ReplaceAll(str, "-", ""))
				if e != nil {
					return fmt.Errorf("%s 无效hex: %w", path, e)
				}
				if dst.Kind() == reflect.Array && len(b) != dst.Len() {
					return fmt.Errorf("%s 需要%d字节", path, dst.Len())
				}
				if dst.Kind() == reflect.Slice {
					dst.SetBytes(b)
				} else {
					reflect.Copy(dst, reflect.ValueOf(b))
				}
				return nil
			}
		}
		list, ok := input.([]any)
		if !ok {
			return fmt.Errorf("%s 需要数组", path)
		}
		if dst.Kind() == reflect.Array && len(list) != dst.Len() {
			return fmt.Errorf("%s 需要%d项", path, dst.Len())
		}
		if dst.Kind() == reflect.Slice {
			dst.Set(reflect.MakeSlice(dst.Type(), len(list), len(list)))
		}
		for i, v := range list {
			if e := yyFillPacketValue(dst.Index(i), v, fmt.Sprintf("%s[%d]", path, i)); e != nil {
				return e
			}
		}
		return nil
	case reflect.Map:
		fields, ok := input.(map[string]any)
		if !ok {
			return fmt.Errorf("%s 需要对象", path)
		}
		dst.Set(reflect.MakeMap(dst.Type()))
		for k, v := range fields {
			key := reflect.New(dst.Type().Key()).Elem()
			value := reflect.New(dst.Type().Elem()).Elem()
			if e := yyFillPacketValue(key, k, path+".key"); e != nil {
				return e
			}
			if e := yyFillPacketValue(value, v, path+"."+k); e != nil {
				return e
			}
			dst.SetMapIndex(key, value)
		}
		return nil
	case reflect.String:
		value, ok := input.(string)
		if !ok {
			return fmt.Errorf("%s 需要字符串", path)
		}
		dst.SetString(value)
		return nil
	case reflect.Bool:
		value, ok := input.(bool)
		if !ok {
			return fmt.Errorf("%s 需要布尔", path)
		}
		dst.SetBool(value)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, e := strconv.ParseInt(fmt.Sprint(input), 10, dst.Type().Bits())
		if e != nil {
			return fmt.Errorf("%s 整数超范围或非整数: %w", path, e)
		}
		dst.SetInt(value)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value, e := strconv.ParseUint(fmt.Sprint(input), 10, dst.Type().Bits())
		if e != nil {
			return fmt.Errorf("%s 无符号整数超范围或非整数: %w", path, e)
		}
		dst.SetUint(value)
		return nil
	case reflect.Float32, reflect.Float64:
		value, e := strconv.ParseFloat(fmt.Sprint(input), dst.Type().Bits())
		if e != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("%s 需要有限数字", path)
		}
		dst.SetFloat(value)
		return nil
	}
	return fmt.Errorf("%s 尚不支持类型 %s", path, dst.Type())
}

func yyMakePacket(kind any, fields map[string]any) (packet.Packet, error) {
	var selected packet.Packet
	var id uint32
	name, byName := kind.(string)
	if byName {
		for _, ctor := range packet.ListAllPackets() {
			pk := ctor()
			if yyPacketName(pk) == name {
				selected = pk
				id = pk.ID()
				break
			}
		}
	}
	if selected == nil {
		n, e := strconv.ParseUint(fmt.Sprint(kind), 10, 32)
		if e != nil {
			return nil, fmt.Errorf("未知包类型 %v，先查询 packet.schema()", kind)
		}
		id = uint32(n)
		if ctor := packet.ListAllPackets()[id]; ctor != nil {
			selected = ctor()
		}
	}
	if selected != nil {
		name = yyPacketName(selected)
	}
	// Session negotiation is owned by the login core, not a plugin send API.
	switch id {
	case packet.IDLogin, packet.IDServerToClientHandshake, packet.IDClientToServerHandshake, packet.IDStartGame, packet.IDNetworkSettings, packet.IDRequestNetworkSettings:
		return nil, fmt.Errorf("%s 属于连接生命周期，不能由插件重发", name)
	}
	if raw, ok := fields["payload_hex"]; ok {
		if len(fields) != 1 {
			return nil, fmt.Errorf("payload_hex必须单独提供完整包体，不能混合结构字段")
		}
		text, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("payload_hex需要字符串")
		}
		body, e := hex.DecodeString(text)
		if e != nil {
			return nil, e
		}
		return &packet.Unknown{PacketID: id, Payload: body}, nil
	}
	if selected == nil {
		return nil, fmt.Errorf("未知ID %d需要payload_hex完整包体", id)
	}
	if e := yyFillPacketValue(reflect.ValueOf(selected).Elem(), fields, name); e != nil {
		return nil, e
	}
	if e := yyValidatePacketBody(selected); e != nil {
		return nil, e
	}
	return selected, nil
}

func yyValidatePacketBody(pk packet.Packet) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("%s字段组合无法编码: %v", yyPacketName(pk), value)
		}
	}()
	var body bytes.Buffer
	pk.Marshal(protocol.NewWriter(&body, 0))
	return nil
}

func yySendPacket(bm *BotManager, args []any) (any, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("sendPacket(typeNameOrId, fields?)")
	}
	fields := map[string]any{}
	if len(args) == 2 {
		var ok bool
		fields, ok = args[1].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("包字段需要对象")
		}
	}
	pk, e := yyMakePacket(args[0], fields)
	if e != nil {
		return nil, e
	}
	if bm == nil || !bm.IsConnected() {
		return nil, fmt.Errorf("目标游戏账号未连接")
	}
	if e = bm.WritePacket(pk); e != nil {
		return nil, e
	}
	return map[string]any{"submitted": true, "confirmed": false, "packet_id": pk.ID(), "packet_type": yyPacketName(pk)}, nil
}

func yyPacketEnvelope(bm *BotManager, data map[string]any) map[string]any {
	name, _ := data["_packet_type"].(string)
	id := uint32(0)
	yyPacketIDsOnce.Do(func() {
		yyPacketIDs = map[string]uint32{}
		for n, ctor := range packet.ListAllPackets() {
			yyPacketIDs[yyPacketName(ctor())] = n
		}
	})
	id = yyPacketIDs[name]
	if name == "Unknown" {
		if n, err := strconv.ParseUint(fmt.Sprint(data["packet_id"]), 10, 32); err == nil {
			id = uint32(n)
		}
	}
	fields := map[string]any{}
	for k, v := range data {
		if k != "_packet_type" {
			fields[k] = v
		}
	}
	index := 0
	display := ""
	if bm != nil {
		bm.mu.Lock()
		index, display = bm.BotIndex, bm.display
		bm.mu.Unlock()
	}
	return map[string]any{"seq": yyPacketSequence.Add(1), "at": time.Now().Format("15:04:05.000"), "direction": "inbound", "bot": fmt.Sprintf("%d:%s", index, display), "packet_type": name, "packet_id": id, "fields": fields}
}

func registerPacketAPI(L *lua.LState) {
	p := L.NewTable()
	L.SetGlobal("packet", p)
	game := L.GetGlobal("game").(*lua.LTable)
	events := L.GetGlobal("events").(*lua.LTable)
	send := L.NewFunction(func(L *lua.LState) int {
		args := []any{luaToGo(L.Get(1))}
		if L.GetTop() > 1 {
			args = append(args, luaToGo(L.Get(2)))
		}
		v, e := yySendPacket(abot(L), args)
		if e != nil {
			L.RaiseError("sendPacket: %v", e)
		}
		L.Push(goToLua(L, v))
		return 1
	})
	all := L.NewFunction(func(L *lua.LState) int {
		pid := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
		handler := L.CheckString(1)
		if pid == "" {
			L.RaiseError("subscribeAll需插件上下文")
			return 0
		}
		if L.GetGlobal("__RE_RUN__") == lua.LTrue {
			return 0
		}
		evMu.Lock()
		for _, old := range evPacketSubs {
			if old.plugin == pid && old.pktType == "*envelope" && old.handler == handler {
				evMu.Unlock()
				return 0
			}
		}
		evMu.Unlock()
		owner := abot(L)
		unsub := packetHubSubscribe("*", func(bm *BotManager, data map[string]any) {
			if bm == owner {
				yyQueueResidentEvent(bm, pid, handler, []any{yyPacketEnvelope(bm, data)})
			}
		})
		evMu.Lock()
		evPacketSubs = append(evPacketSubs, &packetLuaSub{plugin: pid, pktType: "*envelope", handler: handler, unsub: unsub})
		evMu.Unlock()
		return 0
	})
	L.SetField(game, "sendPacket", send)
	L.SetField(events, "subscribeAll", all)
	L.SetField(p, "send", send)
	L.SetField(p, "on", events.RawGetString("subscribe"))
	L.SetField(p, "onAll", all)
	L.SetField(p, "schema", events.RawGetString("packetSchema"))
	L.SetField(p, "describe", L.NewFunction(func(L *lua.LState) int {
		kind := L.CheckString(1)
		for _, row := range yyPacketSchema() {
			if row["name"] == kind || fmt.Sprint(row["id"]) == kind {
				L.Push(goToLua(L, row))
				return 1
			}
		}
		L.RaiseError("未知包类型 %s", kind)
		return 0
	}))
}

func yySDKPacketAlias(method string) string {
	switch method {
	case "packet.send":
		return "game.sendPacket"
	case "packet.on":
		return "events.subscribe"
	case "packet.onAll":
		return "events.subscribeAll"
	case "packet.schema":
		return "events.packetSchema"
	}
	return method
}
