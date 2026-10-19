package main

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestSkinPreviewPNGAndZip(t *testing.T) {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 64, 64)))
	for _, packed := range []bool{false, true} {
		raw := b.Bytes()
		if packed {
			var archive bytes.Buffer
			z := zip.NewWriter(&archive)
			f, _ := z.Create("skins/skin.png")
			_, _ = f.Write(raw)
			_ = z.Close()
			raw = archive.Bytes()
		}
		r, e := decodeSkinPreview(raw)
		if e != nil || r["width"] != 64 || r["height"] != 64 || r["texture"] == "" {
			t.Fatal(r, e)
		}
	}
	if _, e := decodeSkinPreview([]byte("not a skin")); e == nil {
		t.Fatal("invalid image accepted")
	}
	for _, url := range []string{"http://x19.gph.netease.com/a.png", "https://127.0.0.1/a", "https://netease.com.attacker.invalid/a", "https://x19.gph.netease.com:123/a"} {
		if officialSkinURL(url) {
			t.Fatal(url)
		}
	}
	if !officialSkinURL("https://x19.gph.netease.com/a.png") {
		t.Fatal("official source rejected")
	}
}
