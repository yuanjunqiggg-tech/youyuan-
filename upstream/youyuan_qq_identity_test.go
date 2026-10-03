package main

import (
	"path/filepath"
	"testing"
)

func TestOfficialQQIdentityValidation(t *testing.T) {
	c := yyQQConfig{Adapter: "qqofficial"}
	c.Assistant.Admins = []string{"166501528"}
	if yyQQValidateAdminIDs(c) == nil {
		t.Fatal("numeric QQ accepted as official identity")
	}
	c.Adapter = "onebot"
	if yyQQValidateAdminIDs(c) != nil {
		t.Fatal("OneBot numeric identity denied")
	}
	c.Adapter = "qqofficial"
	c.Assistant.Admins = []string{"A1B2-C3D4"}
	if yyQQValidateAdminIDs(c) != nil {
		t.Fatal("openid rejected")
	}
}
func TestAuthorizeOnlySeenOfficialMember(t *testing.T) {
	old, path, seen := yyQQ, yyQQPath, yyQQHelper.Seen
	defer func() { yyQQ = old; yyQQPath = path; yyQQHelper.Seen = seen }()
	yyQQ = yyQQConfig{}
	yyQQPath = filepath.Join(t.TempDir(), "qq.json")
	yyQQHelper.Seen = map[string]string{"real-openid": "member"}
	if yyQQAuthorizeSeen("unseen", "super") == nil {
		t.Fatal("unseen user authorized")
	}
	for _, step := range []struct {
		role  string
		level int
	}{{"admin", 1}, {"super", 2}, {"member", 0}} {
		if err := yyQQAuthorizeSeen("real-openid", step.role); err != nil {
			t.Fatal(err)
		}
		if yyQQRole(yyQQ, "real-openid") != step.level {
			t.Fatal("wrong role")
		}
	}
}
