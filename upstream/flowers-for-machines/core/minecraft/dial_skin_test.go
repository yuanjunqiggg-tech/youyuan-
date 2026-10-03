package minecraft

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"strings"
	"testing"
)

// testSkinPNG 生成一张 64x64 纯色 PNG 供测试使用。
func testSkinPNG(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for i := 0; i < 64*64; i++ {
		img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = 200, 100, 50, 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// testSkinZIP 构造一个网易高级皮肤 ZIP：皮肤 PNG + geometry.json + manifest.json。
func testSkinZIP(t *testing.T, geometryRaw string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	pngFile, _ := zw.Create("content/skin.png")
	pngFile.Write(testSkinPNG(t))
	geoFile, _ := zw.Create("content/geometry.json")
	geoFile.Write([]byte(geometryRaw))
	manifestFile, _ := zw.Create("content/manifest.json")
	manifestFile.Write([]byte(`{"header":{"uuid":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","name":"test"}}`))
	bloomFile, _ := zw.Create("content/_bloom.png")
	bloomFile.Write([]byte{1, 2, 3})
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// TestLoadSkinZIP 验证高级皮肤 ZIP 的提取：
// - 皮肤 PNG 取非 _bloom.png
// - resource patch 指向 geometry.json 自身的几何标识
// - SkinID 使用 manifest UUID
func TestLoadSkinZIP(t *testing.T) {
	geometryRaw := `{"geometry.my4d":{"texturewidth":128,"textureheight":128,"bones":[
		{"name":"waist","pivot":[0,12,0]},
		{"name":"body","pivot":[0,24,0],"cubes":[{"origin":[-4,12,-2],"size":[8,12,4],"uv":[32,32]}]},
		{"name":"head","pivot":[0,24,0],"cubes":[{"origin":[-4,0,-4],"size":[8,8,8],"uv":[0,0]}]},
		{"name":"rightArm","pivot":[-5,22,0],"cubes":[{"origin":[-8,12,-2],"size":[4,12,4],"uv":[80,32]}]},
		{"name":"rightSleeve","pivot":[-5,22,0],"cubes":[{"origin":[-8,12,-2],"size":[4,12,4],"uv":[80,64]}]}
	]}}`

	imageData, geometryData, patch, skinUUID, err := loadSkinZIP(testSkinZIP(t, geometryRaw))
	if err != nil {
		t.Fatalf("loadSkinZIP: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(imageData)); err != nil {
		t.Fatalf("extracted skin is not PNG: %v", err)
	}
	if skinUUID != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Fatalf("skinUUID = %q, want manifest uuid", skinUUID)
	}

	// resource patch 必须指向几何自带的标识
	var patchObj struct {
		Geometry struct {
			Default string `json:"default"`
		} `json:"geometry"`
	}
	if err := json.Unmarshal(patch, &patchObj); err != nil {
		t.Fatalf("patch json: %v", err)
	}
	if patchObj.Geometry.Default != "geometry.my4d" {
		t.Fatalf("patch default = %q, want geometry.my4d", patchObj.Geometry.Default)
	}

	// 骨骼必须被修复：root 兜底、waist/legs→root、head/arms→body、body→waist
	geometryMap := map[string]json.RawMessage{}
	if err := json.Unmarshal(geometryData, &geometryMap); err != nil {
		t.Fatalf("geometry json: %v", err)
	}
	var geometry struct {
		Bones []struct {
			Name   string `json:"name"`
			Parent string `json:"parent"`
		} `json:"bones"`
	}
	raw, ok := geometryMap["geometry.my4d"]
	if !ok {
		t.Fatalf("geometry identifier missing in output")
	}
	if err := json.Unmarshal(raw, &geometry); err != nil {
		t.Fatalf("geometry bones: %v", err)
	}
	wantParent := map[string]string{"waist": "root", "body": "waist", "head": "body", "rightArm": "body"}
	hasRoot := false
	for _, bone := range geometry.Bones {
		if bone.Name == "root" {
			hasRoot = true
		}
		if want, ok := wantParent[bone.Name]; ok && bone.Parent != want {
			t.Fatalf("bone %q parent = %q, want %q", bone.Name, bone.Parent, want)
		}
	}
	if !hasRoot {
		t.Fatalf("root bone not ensured")
	}
}

// TestDownscaleSkin128To64 验证 128x128 RGBA 到 64x64 的 2x2 平均降采样。
func TestDownscaleSkin128To64(t *testing.T) {
	// 128x128 全红像素（A=255）
	p := make([]byte, 128*128*4)
	for i := 0; i < len(p); i += 4 {
		p[i] = 255 // R
		p[i+3] = 255
	}
	out := downscaleSkin128To64(p)
	if len(out) != 64*64*4 {
		t.Fatalf("len(out) = %d, want %d", len(out), 64*64*4)
	}
	// 全红输入应保持全红
	for i := 0; i < len(out); i += 4 {
		if out[i] != 255 || out[i+3] != 255 {
			t.Fatalf("pixel %d: R=%d A=%d, want 255/255", i/4, out[i], out[i+3])
		}
	}
	// 混合输入：2x2 块 (0,255,0,255) 平均值
	p2 := make([]byte, 128*128*4)
	for i := 0; i < len(p2); i += 4 {
		p2[i+3] = 255
	}
	p2[0] = 0
	p2[4] = 0
	p2[128*4] = 0
	p2[128*4+4] = 0 // 左上 2x2 全黑，其余全透明黑
	out2 := downscaleSkin128To64(p2)
	if out2[0] != 0 {
		t.Fatalf("top-left pixel R = %d, want 0", out2[0])
	}
}

// TestEmbeddedSkinPatchConsistent 验证内置默认皮肤的 resource patch 引用的几何标识
// 确实存在于内置几何数据中——标识不匹配会让客户端拒绝皮肤并回退默认皮肤。
func TestEmbeddedSkinPatchConsistent(t *testing.T) {
	var patchObj struct {
		Geometry struct {
			Default string `json:"default"`
		} `json:"geometry"`
	}
	if err := json.Unmarshal(skinResourcePatch, &patchObj); err != nil {
		t.Fatalf("embedded patch json: %v", err)
	}
	identifier := patchObj.Geometry.Default
	if !strings.Contains(string(skinGeometry), `"identifier" : "`+identifier+`"`) {
		t.Fatalf("embedded patch references %q but it does not exist in embedded geometry", identifier)
	}
	// 宽臂与细臂标识都必须存在（isSlim 会切换 patch 标识）
	for _, id := range []string{"geometry.humanoid.custom", "geometry.humanoid.customSlim"} {
		if !strings.Contains(string(skinGeometry), `"identifier" : "`+id+`"`) {
			t.Fatalf("embedded geometry missing identifier %q", id)
		}
	}
}
