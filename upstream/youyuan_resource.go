package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

var yyResourceDocs = map[string]string{
	"plugin.codeDir":         "无参数，当前插件源码根目录。Lua模块可require，模块只在该常驻VM首次require时加载。",
	"plugin.dataDir":         "无参数，当前插件持久数据目录；SDK按账号隔离。旧game.dataDir保持兼容。",
	"plugin.configDir":       "无参数，当前插件配置目录。",
	"plugin.resourceDir":     "无参数；有resources子目录则使用该目录，否则兼容使用代码目录。",
	"plugin.tempDir":         "无参数；插件数据目录下.tmp，可自行删除临时文件。",
	"plugin.readResource":    "relative_path,encoding=utf8；读取资源字符串；二进制跨JSON使用encoding=base64。拒绝绝对路径、越界与符号链接。",
	"plugin.listResources":   "relative_path默认.，递归列出目录和文件，含path/dir/size；不跟随符号链接。",
	"plugin.existsResource":  "relative_path，检查资源是否存在；非法路径报错。",
	"plugin.extractResource": "source_relative,destination_relative默认同源；将资源原字节复制至plugin.dataDir中的目标，创建父目录。不是解压ZIP，不写任意外部路径。",
	"plugin.listOutputs":     "relative_dir默认.；列出当前插件数据目录内生成的文件，返回path/name/bytes/modified_at/download_url，不返回文件内容。",
	"plugin.exportFile":      "relative_file；取得当前插件生成文件的下载信息，保持原格式/原字节，不是备份整个插件。文件须已写入plugin.dataDir。链接需登录当前账号，snapshot=false表示下载当前文件版本。",
}

func init() {
	for method, doc := range yyResourceDocs {
		yyExtraMethods[method] = doc
	}
}

type yyResourceRoots struct{ Code, Data, Config, Kind string }

func (r yyResourceRoots) resourceDir() string {
	path := filepath.Join(r.Code, "resources")
	if st, e := os.Stat(path); e == nil && st.IsDir() {
		return path
	}
	return r.Code
}

func yyResourceCall(r yyResourceRoots, method string, args []any) (any, error) {
	if r.Code == "" {
		return nil, fmt.Errorf("需要有效插件目录")
	}
	switch method {
	case "plugin.codeDir":
		return r.Code, nil
	case "plugin.dataDir":
		return r.Data, os.MkdirAll(r.Data, 0755)
	case "plugin.configDir":
		return r.Config, os.MkdirAll(r.Config, 0755)
	case "plugin.resourceDir":
		return r.resourceDir(), nil
	case "plugin.tempDir":
		p := filepath.Join(r.Data, ".tmp")
		return p, os.MkdirAll(p, 0755)
	case "plugin.listOutputs", "plugin.exportFile":
		name, e := yyArgString(args, 0, ".")
		if e != nil {
			return nil, e
		}
		if method == "plugin.listOutputs" {
			return yyListOutputs(r, name)
		}
		return yyOutputInfo(r, name)
	}
	name, e := yyArgString(args, 0, ".")
	if e != nil {
		return nil, e
	}
	path, e := yyProjectPath(r.resourceDir(), name)
	if e != nil {
		return nil, e
	}
	switch method {
	case "plugin.existsResource":
		_, e := os.Stat(path)
		if os.IsNotExist(e) {
			return false, nil
		}
		return e == nil, e
	case "plugin.listResources":
		rows := []map[string]any{}
		e = filepath.WalkDir(path, func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("资源不允许符号链接: %s", p)
			}
			rel, e := filepath.Rel(r.resourceDir(), p)
			if e != nil {
				return e
			}
			st, e := d.Info()
			if e != nil {
				return e
			}
			rows = append(rows, map[string]any{"path": filepath.ToSlash(rel), "dir": d.IsDir(), "size": st.Size()})
			return nil
		})
		return rows, e
	case "plugin.readResource", "plugin.extractResource":
		data, e := os.ReadFile(path)
		if e != nil {
			return nil, e
		}
		if method == "plugin.readResource" {
			encoding, e := yyArgString(args, 1, "utf8")
			if e != nil {
				return nil, e
			}
			if encoding == "base64" {
				return base64.StdEncoding.EncodeToString(data), nil
			}
			if encoding != "utf8" {
				return nil, fmt.Errorf("encoding为utf8/base64")
			}
			return string(data), nil
		}
		destination, e := yyArgString(args, 1, name)
		if e != nil {
			return nil, e
		}
		target, e := yyProjectPath(r.Data, destination)
		if e != nil {
			return nil, e
		}
		if e = os.MkdirAll(filepath.Dir(target), 0755); e != nil {
			return nil, e
		}
		if e = os.WriteFile(target, data, 0644); e != nil {
			return nil, e
		}
		return target, nil
	}
	return nil, fmt.Errorf("未知资源方法 %s", method)
}

func yyLuaResourceRoots(L *lua.LState) (yyResourceRoots, error) {
	if L.Context() != nil {
		if roots, ok := L.Context().Value(yyLegacyRootsKey{}).(yyResourceRoots); ok {
			return roots, nil
		}
	}
	id := lua.LVAsString(L.GetGlobal("__PLUGIN_ID__"))
	if pluginController == nil || !validPluginID(id) {
		return yyResourceRoots{}, fmt.Errorf("需要插件上下文")
	}
	pluginController.mu.Lock()
	root := pluginController.privateDir
	pluginController.mu.Unlock()
	code, e := pluginController.CodeDir(id)
	if e != nil {
		return yyResourceRoots{}, e
	}
	if explicit := lua.LVAsString(L.GetGlobal("__YY_PLUGIN_CODE_DIR__")); explicit != "" {
		base := filepath.Join(root, "插件文件")
		rel, err := filepath.Rel(base, explicit)
		if err != nil {
			return yyResourceRoots{}, err
		}
		code, err = yyProjectPath(base, rel)
		if err != nil {
			return yyResourceRoots{}, err
		}
	}
	return yyResourceRoots{Code: code, Data: filepath.Join(root, "插件数据文件", id), Config: filepath.Join(root, "插件配置文件", id)}, nil
}

func registerResourceAPI(L *lua.LState) {
	table, ok := L.GetGlobal("plugin").(*lua.LTable)
	if !ok {
		return
	}
	for method := range yyResourceDocs {
		method := method
		L.SetField(table, strings.TrimPrefix(method, "plugin."), L.NewFunction(func(L *lua.LState) int {
			r, e := yyLuaResourceRoots(L)
			if e != nil {
				L.RaiseError("%v", e)
				return 0
			}
			args := []any{}
			for i := 1; i <= L.GetTop(); i++ {
				args = append(args, luaToGo(L.Get(i)))
			}
			v, e := yyResourceCall(r, method, args)
			if e != nil {
				L.RaiseError("%s: %v", method, e)
				return 0
			}
			L.Push(goToLua(L, v))
			return 1
		}))
	}
}

func yyLuaModulePath(L *lua.LState) {
	r, e := yyLuaResourceRoots(L)
	if e != nil {
		return
	}
	pack, ok := L.GetGlobal("package").(*lua.LTable)
	if !ok {
		return
	}
	old := lua.LVAsString(L.GetField(pack, "path"))
	L.SetField(pack, "path", lua.LString(filepath.ToSlash(filepath.Join(r.Code, "?.lua"))+";"+filepath.ToSlash(filepath.Join(r.Code, "?", "init.lua"))+";"+old))
}
