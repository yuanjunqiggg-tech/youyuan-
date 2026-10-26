package nbt_parser_block

import (
	"fmt"
	"math"
	"reflect"
)

// SkullNBT matches the SkullType field found in the beta.12 APK's decoded
// Go type metadata. Production uses normal item placement and cache readback,
// not an assumption that arbitrary BlockActorData writes are accepted.
type SkullNBT struct {
	SkullType byte `mapstructure:"SkullType"`
}
type Skull struct {
	DefaultBlock
	NBT SkullNBT
}

var SkullNames = []string{"minecraft:skeleton_skull", "minecraft:wither_skeleton_skull", "minecraft:zombie_head", "minecraft:player_head", "minecraft:creeper_head", "minecraft:dragon_head", "minecraft:piglin_head"}

func SkullTypeForName(name string) byte {
	for i, n := range SkullNames {
		if n == name {
			return byte(i)
		}
	}
	return 0
}

func (s *Skull) Parse(data map[string]any) error {
	v, ok := data["SkullType"]
	if !ok {
		return nil
	}
	r := reflect.ValueOf(v)
	var n float64
	switch r.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n = float64(r.Uint())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n = float64(r.Int())
	case reflect.Float32, reflect.Float64:
		n = r.Float()
	default:
		return fmt.Errorf("SkullType必须为0..6整数")
	}
	// Modern named heads use 255 as a sentinel: the subtype lives in the
	// block name. Verified from a real wither_skeleton_skull structure read.
	if n == 255 {
		for i, name := range SkullNames {
			if s.Name == name { s.NBT.SkullType = byte(i); return nil }
		}
	}
	if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 6 || math.Trunc(n) != n {
		return fmt.Errorf("SkullType必须为0..6整数")
	}
	s.NBT.SkullType = byte(n)
	return nil
}
func (*Skull) NeedSpecialHandle() bool   { return true }
func (*Skull) NeedCheckCompletely() bool { return true }
func (s *Skull) Format(prefix string) string {
	return s.DefaultBlock.Format(prefix) + fmt.Sprintf("%s头颅类型: %d\n", prefix, s.NBT.SkullType)
}
func (s *Skull) NBTStableBytes() []byte { return []byte{s.NBT.SkullType} }
func (s *Skull) FullStableBytes() []byte {
	return append(s.DefaultBlock.FullStableBytes(), s.NBT.SkullType)
}
