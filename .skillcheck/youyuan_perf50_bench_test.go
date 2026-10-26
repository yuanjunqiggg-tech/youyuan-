package main

import (
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol"
	"github.com/OmineDev/flowers-for-machines/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
	"testing"
)

var yyPerf50Map map[string]any

func yyPerf50Move() *packet.MovePlayer {
	return &packet.MovePlayer{EntityRuntimeID: 99, Position: mgl32.Vec3{1, 64, 3}, Yaw: 45, Pitch: 15, OnGround: true}
}
func yyPerf50Health() *packet.UpdateAttributes {
	return &packet.UpdateAttributes{EntityRuntimeID: 99, Attributes: []protocol.Attribute{{AttributeValue: protocol.AttributeValue{Name: "minecraft:health", Value: 17, Max: 40}}, {AttributeValue: protocol.AttributeValue{Name: "minecraft:movement", Value: .1, Max: 1}}}}
}
func BenchmarkPerf50MapMove(b *testing.B) {
	p := yyPerf50Move()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		yyPerf50Map = packetToMap(p)
	}
}
func BenchmarkPerf50MapHealth(b *testing.B) {
	p := yyPerf50Health()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		yyPerf50Map = packetToMap(p)
	}
}
func BenchmarkPerf50Dispatch(b *testing.B) {
	bot := &BotManager{}
	p := yyPerf50Move()
	stop := packetHubSubscribe("MovePlayer", func(_ *BotManager, data map[string]any) { yyPerf50Map = data })
	defer stop()
	wildcard := packetHubSubscribe("*", func(_ *BotManager, data map[string]any) { yyPerf50Map = data })
	defer wildcard()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		packetHubDispatch(bot, p)
	}
}
func BenchmarkPerf50Unsubscribed(b *testing.B) {
	bot := &BotManager{}
	p := yyPerf50Move()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		packetHubDispatch(bot, p)
	}
}
func BenchmarkPerf50SenseMove(b *testing.B) {
	bot := &BotManager{display: "test", runtimeID: 1}
	p := yyPerf50Move()
	yySenseFor(bot)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		yySenseHandle(bot, p)
	}
}
