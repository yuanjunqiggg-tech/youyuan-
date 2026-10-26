package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func init() { hub = NewSSEHub() }

func doDrawReq(t *testing.T, h http.HandlerFunc, method, body string) map[string]any {
	t.Helper()
	var rd *bytes.Reader
	if body == "" {
		rd = bytes.NewReader(nil)
	} else {
		rd = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, "/api/draw", rd)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h(rr, req)
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("非 JSON 响应: %s", rr.Body.String())
	}
	return out
}

func TestHandlerCreateAndStroke(t *testing.T) {
	// create 透明（批量模式：不异步放置，便于立即验证网格与撤销）
	res := doDrawReq(t, handleDrawCreate, "POST", `{"x1":0,"z1":0,"w":10,"d":6,"y":64,"mode":"transparent","sync":"batch"}`)
	if res["ok"] != true {
		t.Fatalf("create ok=false: %v", res)
	}
	st := res["state"].(map[string]any)
	region := st["region"].(map[string]any)
	if region["w"] != float64(10) || region["d"] != float64(6) {
		t.Fatalf("w,d=%v,%v want 10,6", region["w"], region["d"])
	}

	// stroke
	res = doDrawReq(t, handleDrawStroke, "POST", `{"points":[{"x":1,"z":1,"block":"minecraft:blue_concrete"},{"x":2,"z":2,"block":"minecraft:blue_concrete"}]}`)
	if res["ok"] != true {
		t.Fatalf("stroke ok=false: %v", res)
	}
	grid := res["state"].(map[string]any)["grid"].([]any)
	if grid[1].([]any)[1].(map[string]any)["b"] != "minecraft:blue_concrete" {
		t.Fatalf("grid[1][1] not set: %v", grid[1].([]any)[1])
	}

	// undo → 恢复为空气（透明基线，无机器人时读空气）
	res = doDrawReq(t, handleDrawUndo, "POST", "")
	if res["ok"] != true {
		t.Fatalf("undo ok=false: %v", res)
	}
	grid = res["state"].(map[string]any)["grid"].([]any)
	cell := grid[1].([]any)[1].(map[string]any)
	if cell["b"] != "minecraft:air" {
		t.Fatalf("after undo grid[1][1]=%v want air", cell)
	}
}

func TestHandlerCreateMissing(t *testing.T) {
	res := doDrawReq(t, handleDrawCreate, "POST", `{"mode":"transparent"}`)
	if res["ok"] != false {
		t.Fatalf("missing coords should fail: %v", res)
	}
}

func TestHandlerCreateFill(t *testing.T) {
	res := doDrawReq(t, handleDrawCreate, "POST", `{"x1":0,"z1":0,"w":3,"d":3,"y":64,"mode":"fill","fill_block":"minecraft:red_concrete","sync":"batch"}`)
	if res["ok"] != true {
		t.Fatalf("fill create failed: %v", res)
	}
	grid := res["state"].(map[string]any)["grid"].([]any)
	for x := 0; x < 3; x++ {
		for z := 0; z < 3; z++ {
			cell := grid[x].([]any)[z].(map[string]any)
			if cell["b"] != "minecraft:red_concrete" {
				t.Fatalf("fill grid[%d][%d]=%v want red_concrete", x, z, cell)
			}
		}
	}
}

func TestBlockTextureResolver(t *testing.T) {
	cases := map[string]string{
		"minecraft:blue_concrete": "concrete_blue",
		"minecraft:white_wool":    "wool_colored_white",
		"minecraft:light_gray_concrete": "concrete_silver",
		"minecraft:red_wool":            "wool_colored_red",
		"minecraft:grass_block":         "grass_top",
		"minecraft:stone":               "stone",
	}
	for name, want := range cases {
		got := drawBlockTexture(name)
		if got != want {
			t.Errorf("drawBlockTexture(%s)=%q want %q", name, got, want)
		}
	}
	// terracotta 走 hardened_clay_stained_* 命名
	if got := drawBlockTexture("minecraft:cyan_terracotta"); got != "hardened_clay_stained_cyan" {
		t.Errorf("cyan_terracotta=%q want hardened_clay_stained_cyan", got)
	}
}

func TestHandlerPalette(t *testing.T) {
	res := doDrawReq(t, handleDrawPalette, "GET", "")
	if res["ok"] != true {
		t.Fatalf("palette failed: %v", res)
	}
	pal := res["palette"].([]any)
	if len(pal) == 0 {
		t.Fatalf("palette empty")
	}
	first := pal[0].(map[string]any)
	if first["name"] == "" {
		t.Fatalf("palette entry missing name: %v", first)
	}
}
