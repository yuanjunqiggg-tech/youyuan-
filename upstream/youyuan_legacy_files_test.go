package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyRelativeFilesNeedNoPluginRewrite(t *testing.T) {
	old := pluginController
	pc := NewPluginController()
	pc.SetPluginDirs(t.TempDir(), "")
	pluginController = pc
	defer func() { pluginController = old }()
	id, e := pc.ImportPackage(writeZip(t, map[string]string{
		"manifest.json":     `{"id":"legacy-files"}`,
		"main.lua":          `function check()local f=assert(io.open('notes/n.txt','w'));f:write('saved');f:close();local a=util.read_file('notes/n.txt');local b=util.read_file('ai_prompts/test.txt');local m=dofile('modules/count.lua');util.json_write('state/test.json',{value=m});return a..'|'..b..'|'..tostring(util.json_load('state/test.json').value)end`,
		"modules/count.lua": "return 23", "ai_prompts/test.txt": "prompt",
	}))
	if e != nil {
		t.Fatal(e)
	}
	source, e := os.ReadFile(filepath.Join(pc.privateDir, "插件文件", id, id+".lua"))
	if e != nil {
		t.Fatal(e)
	}
	bm := &BotManager{}
	if e = LoadLuaPlugin(bm, id, string(source)); e != nil {
		t.Fatal(e)
	}
	defer yyStopResident(bm, id)
	v, e := yyCallResident(bm, id, "check", nil, false, false)
	if e != nil || v != "saved|prompt|23" {
		t.Fatal(v, e)
	}
	if data, e := os.ReadFile(filepath.Join(pc.DataDir(id), "notes/n.txt")); e != nil || string(data) != "saved" {
		t.Fatal(string(data), e)
	}
}
