package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"testing"
)

func TestAccountProfilePreservesCredentialsAndUniqueOwner(t *testing.T) {
	oldRoot, oldItems := yyAccounts.Root, yyAccounts.Items
	defer func() { yyAccounts.Root = oldRoot; yyAccounts.Items = oldItems }()
	yyAccounts.Root = t.TempDir()
	yyAccounts.Items = []yyManagedAccount{{ID: "second", Name: "old", Token: "private-token", Server: "test"}}
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	avatar := "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())
	if e := yySaveAccountProfile("second", "独立账号", "owner", &avatar); e != nil {
		t.Fatal(e)
	}
	if yyAccountProfileView("main", "主账号")["profile_role"] != "bot" || yyAccountProfileView("second", "")["profile_role"] != "owner" {
		t.Fatal("owner selection is not unique")
	}
	if yyAccounts.Items[0].Token != "private-token" || yyAccounts.Items[0].Server != "test" {
		t.Fatal("profile changed login credentials")
	}
	if e := yySaveAccountProfile("second", "改名", "bot", nil); e != nil {
		t.Fatal(e)
	}
	if yyAccountProfileView("second", "")["avatar"] != avatar {
		t.Fatal("omitted avatar lost existing image")
	}
	if yyAccountProfileView("main", "")["profile_role"] != "owner" {
		t.Fatal("default owner not restored")
	}
	if e := yySaveAccountProfile("missing", "不存在", "bot", nil); e == nil {
		t.Fatal("unknown profile accepted")
	}
}
func TestAccountAvatarRejectsExecutableAndMalformedInput(t *testing.T) {
	for _, s := range []string{"https://tracking.invalid/photo", "data:image/svg+xml;base64,PHN2Zy8+", "data:image/png;base64,bm90LXBpY3R1cmU=", "javascript:alert(1)"} {
		if yyValidateAccountAvatar(s) == nil {
			t.Fatal("invalid avatar accepted")
		}
	}
}
