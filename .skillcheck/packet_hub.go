package main

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
)

// ── 通用数据包订阅（插件 events.subscribe + AI listen_packet）──
//
// packetHub 按包类型名（Go 类型名，如 "MovePlayer"/"SetTime"）注册订阅回调。
// consumePackets 每收到一个包就调 packetHubDispatch：反射类型名，仅对已订阅类型
// 序列化并分发。惰性订阅——订阅了才监听，符合"机器人只监听已订阅的数据包"。

type packetSubscriber struct {
	id uint64
	cb func(bm *BotManager, data map[string]any)
}

var packetHub = struct {
	sync.Mutex
	subs   map[string][]*packetSubscriber
	nextID uint64
}{subs: map[string][]*packetSubscriber{}}

// Published slices are immutable. Dispatch keeps its snapshot even when a
// callback subscribes/unsubscribes, just like the previous locked copy.
var yyPacketSubscribers atomic.Pointer[map[string][]*packetSubscriber]

func yyPublishPacketSubscribers() {
	next := make(map[string][]*packetSubscriber, len(packetHub.subs))
	for name, list := range packetHub.subs {
		if len(list) > 0 {
			next[name] = append([]*packetSubscriber(nil), list...)
		}
	}
	yyPacketSubscribers.Store(&next)
}

// packetHubSubscribe 订阅某类数据包，返回退订函数。回调在 consumePackets 同步调用，
// 订阅者不得长时间阻塞收包循环（需要的话自行开 goroutine）。
func packetHubSubscribe(typeName string, cb func(bm *BotManager, data map[string]any)) (unsub func()) {
	packetHub.Lock()
	defer packetHub.Unlock()
	packetHub.nextID++
	sub := &packetSubscriber{id: packetHub.nextID, cb: cb}
	packetHub.subs[typeName] = append(packetHub.subs[typeName], sub)
	yyPublishPacketSubscribers()
	return func() {
		packetHub.Lock()
		defer packetHub.Unlock()
		list := packetHub.subs[typeName]
		for i, s := range list {
			if s.id == sub.id {
				packetHub.subs[typeName] = append(list[:i], list[i+1:]...)
				yyPublishPacketSubscribers()
				return
			}
		}
	}
}

// packetHubUnsubscribedCount 查询某类型当前订阅数（测试/诊断用）。
func packetHubUnsubscribedCount(typeName string) int {
	packetHub.Lock()
	defer packetHub.Unlock()
	return len(packetHub.subs[typeName])
}

// packetHubDispatch 收到包时调用：反射类型名，仅对已订阅类型序列化并分发。
func packetHubDispatch(bm *BotManager, pk packet.Packet) {
	tn := reflect.TypeOf(pk)
	if tn == nil || tn.Kind() != reflect.Ptr {
		return
	}
	name := tn.Elem().Name()

	snapshot := yyPacketSubscribers.Load()
	if snapshot == nil {
		return
	}
	named, wildcard := (*snapshot)[name], (*snapshot)["*"]
	if len(named)+len(wildcard) == 0 {
		return
	}

	data := packetToMap(pk)
	data["_packet_type"] = name
	for _, s := range named {
		s.cb(bm, data)
	}
	for _, s := range wildcard {
		s.cb(bm, data)
	}
}

func packetHubWants(name string) bool {
	snapshot := yyPacketSubscribers.Load()
	return snapshot != nil && (len((*snapshot)[name]) > 0 || len((*snapshot)["*"]) > 0)
}

type yyPacketField struct {
	index int
	name  string
}

var yyPacketFields sync.Map // reflect.Type -> immutable exported field metadata
func yyFieldsFor(t reflect.Type) []yyPacketField {
	if cached, ok := yyPacketFields.Load(t); ok {
		return cached.([]yyPacketField)
	}
	fields := make([]yyPacketField, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.IsExported() {
			fields = append(fields, yyPacketField{i, snakeCase(f.Name)})
		}
	}
	cached, _ := yyPacketFields.LoadOrStore(t, fields)
	return cached.([]yyPacketField)
}
func yyPacketStructMap(rv reflect.Value) map[string]any {
	fields := yyFieldsFor(rv.Type())
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		out[f.name] = toPlainValue(rv.Field(f.index))
	}
	return out
}

// packetToMap 用反射把任意数据包序列化为 map[string]any（字段名 snake_case）。
// 完整序列化、不截断：[]byte 转完整 hex，slice/array/map 递归转 []any/map。
func packetToMap(pk any) map[string]any {
	rv := reflect.ValueOf(pk)
	if !rv.IsValid() {
		return map[string]any{}
	}
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return map[string]any{}
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return map[string]any{}
	}
	return yyPacketStructMap(rv)
}

// toPlainValue 把一个 reflect.Value 转成可直接塞进 map 的 Go 值。
func toPlainValue(v reflect.Value) any {
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		return toPlainValue(v.Elem())
	case reflect.Bool:
		return v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	case reflect.Float32, reflect.Float64:
		return v.Float()
	case reflect.String:
		return v.String()
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			// []byte → 完整 hex（不截断）
			return hex.EncodeToString(v.Bytes())
		}
		out := make([]any, v.Len())
		for i := 0; i < v.Len(); i++ {
			out[i] = toPlainValue(v.Index(i))
		}
		return out
	case reflect.Array:
		// [N]byte（如 UUID）→ 完整 hex；其余数组 → []any
		if v.Type().Elem().Kind() == reflect.Uint8 {
			b := make([]byte, v.Len())
			for i := 0; i < v.Len(); i++ {
				b[i] = byte(v.Index(i).Uint())
			}
			return hex.EncodeToString(b)
		}
		out := make([]any, v.Len())
		for i := 0; i < v.Len(); i++ {
			out[i] = toPlainValue(v.Index(i))
		}
		return out
	case reflect.Map:
		out := map[string]any{}
		iter := v.MapRange()
		for iter.Next() {
			out[fmt.Sprintf("%v", iter.Key().Interface())] = toPlainValue(iter.Value())
		}
		return out
	case reflect.Struct:
		return yyPacketStructMap(v)
	case reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return toPlainValue(v.Elem())
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}

// snakeCase 把 CamelCase 字段名转 snake_case（AbcDef → abc_def；连续大写 ABCDef → abc_def）。
func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 && !(s[i-1] >= 'A' && s[i-1] <= 'Z') {
				b.WriteByte('_')
			}
			b.WriteByte(byte(r + 32))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
