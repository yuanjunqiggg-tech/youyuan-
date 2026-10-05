package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

var yyStorageDocs = map[string]string{
	"storage.get":         "key,default?；内存读取并复制值，账号/插件的数据目录独立；首次读取storage.json一次。",
	"storage.set":         "key,value；复制JSON可表示的值到内存，后台约1秒自动原子持久化；重要提交用flush或transaction。",
	"storage.atomicAdd":   "key,delta=1；持锁原子加，默认从0开始；非数字报错，不发生读改写竞争。",
	"storage.delete":      "key；删除缓存键，后台持久化。",
	"storage.flush":       "无参数；同步写临时文件、Sync、原子替换storage.json；失败报真实路径错误并保留dirty。",
	"storage.transaction": "operations数组；op=set/add/delete/check,key,value。check值不符则整批回滚；整批验证后持锁同步持久化成功才替换缓存。不是任意Lua函数/跨文件事务。",
}

type yyStorageCache struct {
	sync.Mutex
	Path      string
	Values    map[string]any
	Dirty     bool
	LastError string
}

var yyStorageCaches = struct {
	sync.Mutex
	items map[string]*yyStorageCache
}{items: map[string]*yyStorageCache{}}
var yyStorageOnce sync.Once

func init() {
	for m, d := range yyStorageDocs {
		yyExtraMethods[m] = d
	}
}

func yyStorageClone(v any) (any, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	var c any
	e = json.Unmarshal(b, &c)
	return c, e
}

func yyStorageFor(dir string) (*yyStorageCache, error) {
	path, e := yyProjectPath(dir, "storage.json")
	if e != nil {
		return nil, e
	}
	yyStorageCaches.Lock()
	defer yyStorageCaches.Unlock()
	if c := yyStorageCaches.items[path]; c != nil {
		return c, nil
	}
	c := &yyStorageCache{Path: path, Values: map[string]any{}}
	if b, e := os.ReadFile(path); e == nil {
		if e = json.Unmarshal(b, &c.Values); e != nil {
			return nil, fmt.Errorf("storage.json: %w", e)
		}
		if c.Values == nil {
			c.Values = map[string]any{}
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	yyStorageCaches.items[path] = c
	yyStorageOnce.Do(func() {
		newSchedJobWith("__storage__", "flush", time.Second, false, nil, func(<-chan struct{}) error {
			yyStorageCaches.Lock()
			rows := make([]*yyStorageCache, 0, len(yyStorageCaches.items))
			for _, v := range yyStorageCaches.items {
				rows = append(rows, v)
			}
			yyStorageCaches.Unlock()
			for _, v := range rows {
				v.Lock()
				if v.Dirty {
					e := v.write(v.Values)
					if e != nil {
						v.LastError = e.Error()
					} else {
						v.Dirty = false
						v.LastError = ""
					}
				}
				v.Unlock()
			}
			return nil
		}, nil)
	})
	return c, nil
}

func (c *yyStorageCache) write(values map[string]any) error {
	data, e := json.Marshal(values)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(c.Path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(c.Path), ".storage-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.Write(data); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, c.Path)
}

func yyStorageCall(dir, method string, args []any) (any, error) {
	c, e := yyStorageFor(dir)
	if e != nil {
		return nil, e
	}
	c.Lock()
	defer c.Unlock()
	if method == "storage.flush" {
		e := c.write(c.Values)
		if e != nil {
			c.LastError = e.Error()
			return nil, e
		}
		c.Dirty = false
		c.LastError = ""
		return true, nil
	}
	if method == "storage.transaction" {
		if len(args) != 1 {
			return nil, fmt.Errorf("transaction需要operations数组")
		}
		ops, ok := args[0].([]any)
		if !ok {
			return nil, fmt.Errorf("transaction需要operations数组")
		}
		copy, e := yyStorageClone(c.Values)
		if e != nil {
			return nil, e
		}
		next := copy.(map[string]any)
		for i, raw := range ops {
			op, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("事务第%d项需对象", i+1)
			}
			k, ok := op["key"].(string)
			if !ok || k == "" {
				return nil, fmt.Errorf("事务第%d项缺key", i+1)
			}
			v, e := yyStorageClone(op["value"])
			if e != nil {
				return nil, e
			}
			switch op["op"] {
			case "set":
				next[k] = v
			case "delete":
				delete(next, k)
			case "check":
				if !reflect.DeepEqual(next[k], v) {
					return nil, fmt.Errorf("事务check失败 key=%s；没有写入", k)
				}
			case "add":
				delta, ok := v.(float64)
				if !ok {
					return nil, fmt.Errorf("事务add需数字")
				}
				old := float64(0)
				if current, exists := next[k]; exists {
					old, ok = current.(float64)
					if !ok {
						return nil, fmt.Errorf("%s不是数字", k)
					}
				}
				next[k] = old + delta
			default:
				return nil, fmt.Errorf("不支持事务操作 %v", op["op"])
			}
		}
		if e := c.write(next); e != nil {
			return nil, e
		}
		c.Values = next
		c.Dirty = false
		return true, nil
	}
	key, e := yySDKString(args, 0)
	if e != nil {
		return nil, e
	}
	switch method {
	case "storage.get":
		v, ok := c.Values[key]
		if !ok && len(args) > 1 {
			v = args[1]
		}
		return yyStorageClone(v)
	case "storage.set":
		if len(args) != 2 {
			return nil, fmt.Errorf("set需要key,value")
		}
		v, e := yyStorageClone(args[1])
		if e != nil {
			return nil, e
		}
		c.Values[key] = v
		c.Dirty = true
		return true, nil
	case "storage.delete":
		_, existed := c.Values[key]
		delete(c.Values, key)
		c.Dirty = true
		return existed, nil
	case "storage.atomicAdd":
		delta := float64(1)
		if len(args) > 1 {
			var ok bool
			delta, ok = args[1].(float64)
			if !ok {
				return nil, fmt.Errorf("delta需数字")
			}
		}
		old := float64(0)
		if v, exists := c.Values[key]; exists {
			var ok bool
			old, ok = v.(float64)
			if !ok {
				return nil, fmt.Errorf("%s不是数字", key)
			}
		}
		v, e := yyStorageClone(old + delta)
		if e != nil {
			return nil, e
		}
		c.Values[key] = v
		c.Dirty = true
		return v, nil
	}
	return nil, fmt.Errorf("未知Storage方法 %s", method)
}

func registerStorageAPI(L *lua.LState) {
	t := L.NewTable()
	L.SetGlobal("storage", t)
	for method := range yyStorageDocs {
		method := method
		L.SetField(t, method[len("storage."):], L.NewFunction(func(L *lua.LState) int {
			r, e := yyLuaResourceRoots(L)
			if e != nil {
				L.RaiseError("%v", e)
				return 0
			}
			a := []any{}
			for i := 1; i <= L.GetTop(); i++ {
				a = append(a, luaToGo(L.Get(i)))
			}
			v, e := yyStorageCall(r.Data, method, a)
			if e != nil {
				L.RaiseError("%s: %v", method, e)
				return 0
			}
			L.Push(goToLua(L, v))
			return 1
		}))
	}
}
