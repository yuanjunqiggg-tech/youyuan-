package main

import (
	lua "github.com/yuin/gopher-lua"
	"os"
	"path/filepath"
	"strings"
)

type yyLegacyRootsKey struct{}

// Resolve legacy paths without changing process cwd (several plugins/accounts
// run concurrently). Absolute paths retain the existing host/OS permissions.
func yyLegacyFilePath(L *lua.LState, name string, write bool) string {
	if filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return name
	}
	roots, e := yyLuaResourceRoots(L)
	if e != nil {
		return name
	}
	normal := filepath.FromSlash(strings.ReplaceAll(name, "\\", "/"))
	parts := strings.Split(filepath.ToSlash(normal), "/")
	if len(parts) > 1 {
		switch parts[0] {
		case "插件文件", "插件数据文件", "插件配置文件":
			// Original scripts build these paths themselves; map them to the
			// configured installation root, not Linux's /opt working directory.
			return filepath.Join(filepath.Dir(filepath.Dir(roots.Code)), normal)
		}
	}
	if !write {
		// Preserve an existing explicit host-relative file first.
		for _, p := range []string{name, filepath.Join(roots.Data, normal), filepath.Join(roots.Code, normal), filepath.Join(roots.resourceDir(), normal)} {
			if st, e := os.Stat(p); e == nil && !st.IsDir() {
				return p
			}
		}
	}
	return filepath.Join(roots.Data, normal)
}

func yyLegacyFileBindings(L *lua.LState) {
	wrap := func(table *lua.LTable, key string, open bool) {
		original, ok := L.GetField(table, key).(*lua.LFunction)
		if !ok {
			return
		}
		L.SetField(table, key, L.NewFunction(func(L *lua.LState) int {
			args := []lua.LValue{}
			for i := 1; i <= L.GetTop(); i++ {
				args = append(args, L.Get(i))
			}
			if len(args) > 0 && args[0].Type() == lua.LTString {
				write := false
				if open && len(args) > 1 {
					mode := lua.LVAsString(args[1])
					write = strings.ContainsAny(mode, "wa+")
				}
				path := yyLegacyFilePath(L, lua.LVAsString(args[0]), write)
				if write {
					if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
						L.RaiseError("io.open: %v", e)
						return 0
					}
				}
				args[0] = lua.LString(path)
			}
			L.SetTop(0)
			if e := L.CallByParam(lua.P{Fn: original, NRet: lua.MultRet, Protect: true}, args...); e != nil {
				L.RaiseError("%v", e)
				return 0
			}
			return L.GetTop()
		}))
	}
	if ioTable, ok := L.GetGlobal("io").(*lua.LTable); ok {
		wrap(ioTable, "open", true)
	}
	wrap(L.G.Global, "dofile", false)
	wrap(L.G.Global, "loadfile", false)
}
