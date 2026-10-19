package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type yyAccountProfile struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}
type yyAccountProfileStore struct {
	OwnerID  string                      `json:"owner_account_id"`
	Profiles map[string]yyAccountProfile `json:"profiles"`
}

var yyAccountProfileMu sync.Mutex

func yyReadAccountProfiles() (yyAccountProfileStore, error) {
	s := yyAccountProfileStore{OwnerID: "main", Profiles: map[string]yyAccountProfile{}}
	if yyAccounts.Root == "" {
		return s, fmt.Errorf("账号资料目录尚未就绪")
	}
	raw, e := os.ReadFile(filepath.Join(yyAccounts.Root, "account-profiles.json"))
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return s, e
	}
	if e = json.Unmarshal(raw, &s); e != nil {
		return s, fmt.Errorf("账号资料文件无法读取，原文件已保留")
	}
	if s.Profiles == nil {
		s.Profiles = map[string]yyAccountProfile{}
	}
	if s.OwnerID == "" {
		s.OwnerID = "main"
	}
	return s, nil
}
func yyAccountProfileView(id, name string) map[string]any {
	yyAccountProfileMu.Lock()
	defer yyAccountProfileMu.Unlock()
	s, _ := yyReadAccountProfiles()
	p := s.Profiles[id]
	if p.Name != "" {
		name = p.Name
	}
	role := "bot"
	if s.OwnerID == id {
		role = "owner"
	}
	return map[string]any{"id": id, "name": name, "avatar": p.Avatar, "profile_role": role, "role_source":"local_profile", "game_server_owner_known": false}
}
func yyValidateAccountAvatar(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > 140000 {
		return fmt.Errorf("头像过大，请选择小于96KB的PNG或JPEG图片")
	}
	prefix, encoded, ok := strings.Cut(value, ",")
	if !ok || (prefix != "data:image/png;base64" && prefix != "data:image/jpeg;base64") {
		return fmt.Errorf("头像只支持PNG或JPEG图片")
	}
	b, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil || len(b) > 96*1024 {
		return fmt.Errorf("头像格式无效或超过96KB")
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 512 || cfg.Height > 512 {
		return fmt.Errorf("头像需为512×512以内的有效图片")
	}
	if (format == "png") != (prefix == "data:image/png;base64") {
		return fmt.Errorf("头像内容与图片类型不匹配")
	}
	return nil
}
func yySaveAccountProfile(id, name, role string, avatar *string) error {
	if id != "main" {
		if _, ok := yyManagedFind(id); !ok {
			return fmt.Errorf("账号不存在")
		}
	}
	name = strings.TrimSpace(name)
	if len([]rune(name)) < 1 || len([]rune(name)) > 48 {
		return fmt.Errorf("显示名称需为1–48个字符")
	}
	if role != "owner" && role != "bot" {
		return fmt.Errorf("请选择主控账号或机器人账号")
	}
	if avatar != nil {
		if e := yyValidateAccountAvatar(*avatar); e != nil {
			return e
		}
	}
	yyAccountProfileMu.Lock()
	defer yyAccountProfileMu.Unlock()
	s, e := yyReadAccountProfiles()
	if e != nil {
		return e
	}
	p := s.Profiles[id]
	p.Name = name
	if avatar != nil {
		p.Avatar = *avatar
	}
	s.Profiles[id] = p
	if role == "owner" {
		s.OwnerID = id
	} else if s.OwnerID == id {
		s.OwnerID = "main"
	}
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	dest := filepath.Join(yyAccounts.Root, "account-profiles.json")
	if e = os.WriteFile(dest+".tmp", b, 0600); e != nil {
		return e
	}
	return os.Rename(dest+".tmp", dest)
}
