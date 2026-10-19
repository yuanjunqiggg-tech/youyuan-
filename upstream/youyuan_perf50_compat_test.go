package main

import (
	"fmt"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"reflect"
	"sync"
	"testing"
)

// Independent previous codec retained only as an oracle for wire/API compatibility.
func yyPerf50OldMap(pk any) map[string]any {
	out := map[string]any{}
	v := reflect.ValueOf(pk)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return out
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return out
	}
	for i := 0; i < v.NumField(); i++ {
		f := v.Type().Field(i)
		if f.IsExported() {
			out[snakeCase(f.Name)] = yyPerf50OldValue(v.Field(i))
		}
	}
	return out
}
func yyPerf50OldValue(v reflect.Value) any {
	if v.Kind() == reflect.Struct {
		return yyPerf50OldMap(v.Interface())
	}
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		return yyPerf50OldValue(v.Elem())
	}
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return toPlainValue(v)
		}
		a := make([]any, v.Len())
		for i := range a {
			a[i] = yyPerf50OldValue(v.Index(i))
		}
		return a
	}
	if v.Kind() == reflect.Map {
		a := map[string]any{}
		it := v.MapRange()
		for it.Next() {
			a[fmt.Sprint(it.Key().Interface())] = yyPerf50OldValue(it.Value())
		}
		return a
	}
	return toPlainValue(v)
}
func TestPerf50EveryRegisteredPacketKeepsFullFieldShape(t *testing.T) {
	for id, newPacket := range packet.ListAllPackets() {
		p := newPacket()
		old, next := yyPerf50OldMap(p), packetToMap(p)
		if !reflect.DeepEqual(old, next) {
			t.Fatalf("packet=%d %T changed fields: old=%v new=%v", id, p, old, next)
		}
	}
	p := yyPerf50Move()
	p.EntityRuntimeID = uint64(1) << 60
	if !reflect.DeepEqual(yyPerf50OldMap(p), packetToMap(p)) {
		t.Fatal("large IDs or coordinates changed")
	}
	p2 := yyPerf50Health()
	if !reflect.DeepEqual(yyPerf50OldMap(p2), packetToMap(p2)) {
		t.Fatal("health attributes changed")
	}
}
func TestPerf50SubscriberChangesDuringDispatchKeepOriginalOrder(t *testing.T) {
	b := &BotManager{}
	got := []int{}
	var remove func()
	first := packetHubSubscribe("SetTime", func(_ *BotManager, _ map[string]any) { got = append(got, 1); remove() })
	defer first()
	remove = packetHubSubscribe("SetTime", func(_ *BotManager, _ map[string]any) { got = append(got, 2) })
	defer remove()
	wild := packetHubSubscribe("*", func(_ *BotManager, _ map[string]any) { got = append(got, 3) })
	defer wild()
	packetHubDispatch(b, &packet.SetTime{})
	if !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatal(got)
	}
	got = nil
	packetHubDispatch(b, &packet.SetTime{})
	if !reflect.DeepEqual(got, []int{1, 3}) {
		t.Fatal(got)
	}
}
func TestPerf50ConcurrentSubscribeAndReceive(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				stop := packetHubSubscribe("SetTime", func(_ *BotManager, _ map[string]any) {})
				packetHubDispatch(&BotManager{}, &packet.SetTime{Time: 1})
				stop()
			}
		}()
	}
	wg.Wait()
}
