package main

// 按账号取皮肤的回归测试：缓存命中、云端凭据兜底、批量预热跳过已缓存、
// 以及缺参数时的明确报错。
//
// 这几条对应线上真实症状：磁盘里已经有纹理，但前端只会用「玩家名」查，
// 名字对不上就永远 404，看起来像「皮肤没下载」。

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// yySkinTestDisk 把磁盘缓存指到临时目录；测试结束后还原，避免污染其它用例。
func yySkinTestDisk(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := yySkinDiskRoot
	yySkinDiskRoot = dir
	t.Cleanup(func() { yySkinDiskRoot = old })
	return dir
}

// yySkinTestTexture 造一张 64x64 的 RGBA 纹理（尺寸符合皮肤规则）。
func yySkinTestTexture() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 4), G: uint8(y * 4), B: 90, A: 255})
		}
	}
	return img.Pix
}

// yySkinTestPutTexture 直接写盘，模拟「预热已经跑过」。
func yySkinTestPutTexture(t *testing.T, name string) {
	t.Helper()
	yySkinDiskWrite(yySkinDiskJob{Name: name, Data: yySkinTestTexture(), Width: 64, Height: 64})
	if _, _, ok := yySkinCacheHas(name); !ok {
		t.Fatalf("写盘后应能 stat 到 %s 的纹理", name)
	}
}

func yySkinTestAccounts(t *testing.T, items []yyManagedAccount) {
	t.Helper()
	oldItems, oldRoot := yyAccounts.Items, yyAccounts.Root
	yyAccounts.Items = items
	yyAccounts.Root = t.TempDir()
	t.Cleanup(func() {
		yyAccounts.Items = oldItems
		yyAccounts.Root = oldRoot
	})
}

// 1. 本地账号：磁盘里有纹理时按账号 ID 直接返回 PNG，不需要玩家名、不联网。
func TestSkinAccountTextureServesCachedByAccountID(t *testing.T) {
	yySkinTestDisk(t)
	yySkinTestAccounts(t, []yyManagedAccount{{ID: "yy_abc", Name: "Steve", AuthURL: "https://auth.example.com"}})
	yySkinTestPutTexture(t, "Steve")

	req := httptest.NewRequest(http.MethodGet, "/api/skin/account-texture?id=yy_abc", nil)
	rec := httptest.NewRecorder()
	handleSkinAccountTexture(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d：%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("Content-Type 异常：%q", got)
	}
	img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("返回内容不是 PNG：%v", err)
	}
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
		t.Fatalf("纹理尺寸异常：%v", img.Bounds())
	}
}

// 2. 仅云端凭据（没有本地账号）也要能取到：用 yy_<凭据ID> 的约定匹配。
func TestSkinAccountResolveCredentialFallback(t *testing.T) {
	yySkinTestDisk(t)
	yySkinTestAccounts(t, nil)
	yySkinTestPutTexture(t, "yy_cred1")

	req := httptest.NewRequest(http.MethodGet, "/api/skin/account-texture?cred=cred1", nil)
	rec := httptest.NewRecorder()
	handleSkinAccountTexture(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("云端凭据应能命中缓存，实际 %d：%s", rec.Code, rec.Body.String())
	}
}

// 3. 头像接口从同一张纹理裁出 16x16。
func TestSkinAccountHeadCropsFromTexture(t *testing.T) {
	yySkinTestDisk(t)
	yySkinTestAccounts(t, []yyManagedAccount{{ID: "yy_head", Name: "Alex", AuthURL: "https://auth.example.com"}})
	yySkinTestPutTexture(t, "Alex")

	req := httptest.NewRequest(http.MethodGet, "/api/skin/account-head?id=yy_head", nil)
	rec := httptest.NewRecorder()
	handleSkinAccountHead(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d：%s", rec.Code, rec.Body.String())
	}
	img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("返回内容不是 PNG：%v", err)
	}
	if img.Bounds().Dx() != 16 || img.Bounds().Dy() != 16 {
		t.Fatalf("头像应为 16x16，实际 %v", img.Bounds())
	}
}

// 4. 没有缓存又没有可用来源时，必须给出可读原因，不能返回一张空图。
func TestSkinAccountTextureReportsReason(t *testing.T) {
	yySkinTestDisk(t)
	yySkinTestAccounts(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/skin/account-texture", nil)
	rec := httptest.NewRecorder()
	handleSkinAccountTexture(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("缺参数走 JSON 提示，状态码应为 200，实际 %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"ok":false`)) {
		t.Fatalf("应返回 ok:false，实际 %s", rec.Body.String())
	}
}

// 5. 批量预热：已经落盘的账号直接跳过，不会为了它去问认证服务。
func TestAccountPrimeSkinsSkipsCached(t *testing.T) {
	yySkinTestDisk(t)
	yySkinTestAccounts(t, []yyManagedAccount{{ID: "yy_warm", Name: "Herobrine", AuthURL: "https://auth.example.com"}})
	yySkinTestPutTexture(t, "Herobrine")

	primed, skipped, failed := yyAccountPrimeSkins([]yyManagedAccount{{ID: "yy_warm", Name: "Herobrine"}})
	if primed != 0 || failed != 0 || skipped != 1 {
		t.Fatalf("已缓存账号应只算跳过，实际 primed=%d skipped=%d failed=%d", primed, skipped, failed)
	}
}

// 6. 账号摘要用的存在性检查只读元数据，且与写盘结果一致。
func TestSkinCacheHasMatchesWrite(t *testing.T) {
	dir := yySkinTestDisk(t)
	yySkinTestPutTexture(t, "Notch")
	if w, h, ok := yySkinCacheHas("Notch"); !ok || w != 64 || h != 64 {
		t.Fatalf("应读到 64x64，实际 %dx%d ok=%v", w, h, ok)
	}
	if _, _, ok := yySkinCacheHas("Missing"); ok {
		t.Fatal("没写过的名字不应命中")
	}
	// 元数据在但像素被删掉时必须算未命中，否则接口会返回一张空图。
	key := yySkinDiskKey("Notch")
	if err := os.Remove(filepath.Join(dir, key+".rgba")); err != nil {
		t.Fatalf("删除像素失败：%v", err)
	}
	if _, _, ok := yySkinCacheHas("Notch"); ok {
		t.Fatal("像素缺失时不应命中")
	}
}
