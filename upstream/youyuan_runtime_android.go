//go:build android

package main

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdint.h>
#include <stdlib.h>
extern char* YouYuanNativeAPICall(char*,char*);
typedef char* (*yy_api_fn)(char*,char*);
typedef void* (*yy_create_fn)(const char*, yy_api_fn);
typedef void* (*yy_create_v2_fn)(const char*, yy_api_fn);
typedef char* (*yy_event_fn)(void*, const char*);
typedef void (*yy_destroy_fn)(void*);
typedef struct {void* library; void* instance; yy_event_fn event; yy_destroy_fn destroy;} yy_handle;
static yy_handle* yy_open(const char* path,const char* account,const char* context){
 void* lib=dlopen(path,RTLD_NOW|RTLD_LOCAL);if(!lib)return NULL;
 yy_create_fn create=(yy_create_fn)dlsym(lib,"yy_create");yy_event_fn event=(yy_event_fn)dlsym(lib,"yy_event");yy_destroy_fn destroy=(yy_destroy_fn)dlsym(lib,"yy_destroy");
 yy_create_v2_fn create_v2=(yy_create_v2_fn)dlsym(lib,"yy_create_v2");
 if((!create&&!create_v2)||!event||!destroy){dlclose(lib);return NULL;}
 yy_handle* h=(yy_handle*)calloc(1,sizeof(yy_handle));h->library=lib;h->instance=create_v2?create_v2(context,YouYuanNativeAPICall):create(account,YouYuanNativeAPICall);h->event=event;h->destroy=destroy;if(!h->instance){dlclose(lib);free(h);return NULL;}return h;
}
static char* yy_invoke(yy_handle* h,const char* event){return h->event(h->instance,event);}
static void yy_close(yy_handle* h){h->destroy(h->instance);dlclose(h->library);free(h);}
*/
import "C"
import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unsafe"
)

var yyNativeMu sync.Mutex
var yyNativeHandles = map[uint64]*C.yy_handle{}
var yyNativeSequence uint64

func yyNativeStart(entry, account, pluginID string) (uint64, error) {
	root, err := os.ReadFile(filepath.Join(appDataDir, "native-directory"))
	if err != nil {
		return 0, err
	}
	path := filepath.Join(strings.TrimSpace(string(root)), entry)
	if filepath.IsAbs(entry) {
		relative, e := filepath.Rel(yyRuntimeRoot, entry)
		if e != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return 0, fmt.Errorf("原生入口必须位于插件目录")
		}
		path = entry
		// Immutable filenames avoid dlopen reusing an older live mapping after publishing.
		binary, e := os.ReadFile(entry)
		if e != nil {
			return 0, e
		}
		artifactDir := filepath.Join(filepath.Dir(entry), ".native")
		if e = os.MkdirAll(artifactDir, 0700); e != nil {
			return 0, e
		}
		path = filepath.Join(artifactDir, fmt.Sprintf("%x.so", sha256.Sum256(binary)))
		if _, e = os.Stat(path); os.IsNotExist(e) {
			if e = os.WriteFile(path, binary, 0600); e != nil {
				return 0, e
			}
		}
	}
	name := C.CString(path)
	defer C.free(unsafe.Pointer(name))
	owner := C.CString(account)
	defer C.free(unsafe.Pointer(owner))
	metadata, _ := json.Marshal(map[string]any{"account": account, "plugin_id": pluginID, "sdk_version": "1.1"})
	contextJSON := C.CString(string(metadata))
	defer C.free(unsafe.Pointer(contextJSON))
	h := C.yy_open(name, owner, contextJSON)
	if h == nil {
		return 0, fmt.Errorf("C++ 插件加载失败，检查 ABI 与 yy_create/yy_event/yy_destroy 导出")
	}
	yyNativeMu.Lock()
	defer yyNativeMu.Unlock()
	yyNativeSequence++
	yyNativeHandles[yyNativeSequence] = h
	return yyNativeSequence, nil
}
func yyNativeEvent(id uint64, event string) (string, error) {
	yyNativeMu.Lock()
	h := yyNativeHandles[id]
	yyNativeMu.Unlock()
	if h == nil {
		return "", fmt.Errorf("原生实例已停止")
	}
	data := C.CString(event)
	defer C.free(unsafe.Pointer(data))
	result := C.yy_invoke(h, data)
	if result == nil {
		return "", fmt.Errorf("C++ 插件返回空指针")
	}
	defer C.free(unsafe.Pointer(result))
	return C.GoString(result), nil
}
func yyNativeStop(id uint64) {
	yyNativeMu.Lock()
	h := yyNativeHandles[id]
	delete(yyNativeHandles, id)
	yyNativeMu.Unlock()
	if h != nil {
		C.yy_close(h)
	}
}

type yyPythonDispatchResult struct {
	Text string
	Err  error
}
type yyPythonDispatchTask struct {
	Data   string
	Stop   bool
	Result chan yyPythonDispatchResult
}
type yyPythonWorker struct {
	Tasks chan yyPythonDispatchTask
	Done  chan struct{}
}

var yyPythonWorkers = struct {
	sync.Mutex
	Items map[string]*yyPythonWorker
}{Items: map[string]*yyPythonWorker{}}

func yyPythonRuntime(action string, params map[string]any) (string, error) {
	params["action"] = action
	params["port"] = yyCorePort
	data, e := json.Marshal(params)
	if e != nil {
		return "", e
	}
	key, _ := params["key"].(string)
	yyPythonWorkers.Lock()
	worker := yyPythonWorkers.Items[key]
	if worker == nil {
		worker = &yyPythonWorker{Tasks: make(chan yyPythonDispatchTask), Done: make(chan struct{})}
		yyPythonWorkers.Items[key] = worker
		go func(w *yyPythonWorker) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			defer close(w.Done)
			defer func() {
				yyPythonWorkers.Lock()
				if yyPythonWorkers.Items[key] == w {
					delete(yyPythonWorkers.Items, key)
				}
				yyPythonWorkers.Unlock()
			}()
			attached := yyPythonThreadEnter()
			if attached == nil {
				defer yyPythonThreadLeave()
			}
			for task := range w.Tasks {
				reply := yyPythonDispatchResult{Err: attached}
				if attached == nil {
					reply.Text, reply.Err = callAndroidSystem("pythonPlugin", task.Data)
				}
				task.Result <- reply
				if task.Stop {
					return
				}
			}
		}(worker)
	}
	yyPythonWorkers.Unlock()
	task := yyPythonDispatchTask{Data: string(data), Stop: action == "stop", Result: make(chan yyPythonDispatchResult, 1)}
	select {
	case worker.Tasks <- task:
	case <-worker.Done:
		return "", fmt.Errorf("Python 工作线程已停止")
	}
	reply := <-task.Result
	if action == "stop" {
		<-worker.Done
	}
	return reply.Text, reply.Err
}
func yyPythonValidate(content, path string) (string, error) {
	return yyPythonRuntime("validate", map[string]any{"key": "validate", "content": content, "path": path})
}

//export YouYuanNativeAPICall
func YouYuanNativeAPICall(path, body *C.char) *C.char {
	target := C.GoString(path)
	if strings.HasPrefix(target, "/api/youyuan/runtime/") {
		return C.CString(`{"ok":false,"error":"禁止原生实例递归调用自己的调度器"}`)
	}
	method := "GET"
	var value any
	if text := C.GoString(body); text != "" {
		method = "POST"
		if err := json.Unmarshal([]byte(text), &value); err != nil {
			return C.CString(`{"ok":false,"error":"JSON 参数无效"}`)
		}
	}
	_, result, err := yyInvoke(method, target, nil, value)
	if err != nil {
		result, _ = json.Marshal(map[string]any{"ok": false, "error": err.Error()})
	}
	return C.CString(string(result))
}
