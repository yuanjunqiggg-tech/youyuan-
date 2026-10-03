//go:build android

package main

/*
#cgo LDFLAGS: -llog
#include <stdlib.h>
#include <string.h>
#include <jni.h>
#include <android/log.h>
static void yyStartupLog(const char* text) { __android_log_write(ANDROID_LOG_INFO, "YouYuanBackend", text); }

static const char* getJString(JNIEnv *env, jstring str) {
    return (*env)->GetStringUTFChars(env, str, NULL);
}

static void freeJString(JNIEnv *env, jstring str, const char *cstr) {
    (*env)->ReleaseStringUTFChars(env, str, cstr);
}

static JavaVM* g_vm = NULL;
// FloatingBridge 的全局引用。必须在 GoMain 所在线程（应用类加载器可见处）缓存，
// 不能从后台 Go 协程 AttachCurrentThread 后 FindClass —— 那会用系统类加载器，
// 找不到 APK 里的应用类，抛 ClassNotFoundException。
static jclass g_floatingBridgeCls = NULL;

static void captureVM(JNIEnv *env) {
    if (g_vm == NULL) { (*env)->GetJavaVM(env, &g_vm); }
    if (g_floatingBridgeCls == NULL) {
        // GoMain 是 MainActivity 的方法，此处 FindClass 用应用类加载器，能命中 FloatingBridge。
        jclass c = (*env)->FindClass(env, "org/youyuan/toolbox/FloatingBridge");
        if (c != NULL) {
            g_floatingBridgeCls = (*env)->NewGlobalRef(env, c);
            (*env)->DeleteLocalRef(env, c);
        }
    }
}

// 从 Go 协程把一条 JSON 消息推给 Java 静态方法 FloatingBridge.emit(String)。
// 使用 captureVM 缓存的全局类引用，避免在后台线程 FindClass 导致的 ClassNotFoundException。
static void callFloatingBridge(const char* json) {
    if (g_vm == NULL || g_floatingBridgeCls == NULL) return;
    JNIEnv* env = NULL;
    if ((*g_vm)->AttachCurrentThread(g_vm, &env, NULL) != JNI_OK) return;
    jmethodID mid = (*env)->GetStaticMethodID(env, g_floatingBridgeCls, "emit", "(Ljava/lang/String;)V");
    if (mid != NULL) {
        jstring s = (*env)->NewStringUTF(env, json);
        (*env)->CallStaticVoidMethod(env, g_floatingBridgeCls, mid, s);
        (*env)->DeleteLocalRef(env, s);
    }
    (*g_vm)->DetachCurrentThread(g_vm);
}

// 从 Go 协程请求 Java 启动指定包名的应用：FloatingBridge.launchApp(String)。
static void callLaunchApp(const char* pkg) {
    if (g_vm == NULL || g_floatingBridgeCls == NULL) return;
    JNIEnv* env = NULL;
    if ((*g_vm)->AttachCurrentThread(g_vm, &env, NULL) != JNI_OK) return;
    jmethodID mid = (*env)->GetStaticMethodID(env, g_floatingBridgeCls, "launchApp", "(Ljava/lang/String;)V");
    if (mid != NULL) {
        jstring s = (*env)->NewStringUTF(env, pkg);
        (*env)->CallStaticVoidMethod(env, g_floatingBridgeCls, mid, s);
        (*env)->DeleteLocalRef(env, s);
    }
    (*g_vm)->DetachCurrentThread(g_vm);
}

// 从 Go 协程同步调用 Java FloatingBridge.call(method, params)，返回 Java 侧 JSON 结果字符串。
// 返回的 const char* 由调用方 C.free 释放；失败返回 strdup("")。
static const char* callJavaSystem(const char* method, const char* params) {
    if (g_vm == NULL || g_floatingBridgeCls == NULL) return strdup("");
    JNIEnv* env = NULL;
    int attached=0;
    if ((*g_vm)->GetEnv(g_vm,(void**)&env,JNI_VERSION_1_6)!=JNI_OK){if ((*g_vm)->AttachCurrentThread(g_vm, &env, NULL) != JNI_OK) return strdup("");attached=1;}
    jmethodID mid = (*env)->GetStaticMethodID(env, g_floatingBridgeCls, "call", "(Ljava/lang/String;Ljava/lang/String;)Ljava/lang/String;");
    if (mid == NULL) { if(attached)(*g_vm)->DetachCurrentThread(g_vm); return strdup(""); }
    jstring m = (*env)->NewStringUTF(env, method);
    jstring p = (*env)->NewStringUTF(env, params);
    jstring result = (jstring)(*env)->CallStaticObjectMethod(env, g_floatingBridgeCls, mid, m, p);
    (*env)->DeleteLocalRef(env, m);
    (*env)->DeleteLocalRef(env, p);
    if (result == NULL) { if(attached)(*g_vm)->DetachCurrentThread(g_vm); return strdup(""); }
    const char* cstr = (*env)->GetStringUTFChars(env, result, NULL);
    char* dup = cstr ? strdup(cstr) : strdup("");
    if (cstr) (*env)->ReleaseStringUTFChars(env, result, cstr);
    (*env)->DeleteLocalRef(env, result);
    if(attached)(*g_vm)->DetachCurrentThread(g_vm);
    return dup;
}
static int yyAttachWorker(){JNIEnv* env=NULL;if(!g_vm)return -1;return (*g_vm)->AttachCurrentThread(g_vm,&env,NULL);}
static void yyDetachWorker(){if(g_vm)(*g_vm)->DetachCurrentThread(g_vm);}
*/
import "C"
import (
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"unsafe"
)

var appDataDir string

//export Java_org_youyuan_toolbox_MainActivity_setDataDir
func Java_org_youyuan_toolbox_MainActivity_setDataDir(env *C.JNIEnv, clazz C.jclass, dir C.jstring) {
	cstr := C.getJString(env, dir)
	appDataDir = C.GoString(cstr)
	C.freeJString(env, dir, cstr)
}

//export Java_org_youyuan_toolbox_MainActivity_setSignatureHash
func Java_org_youyuan_toolbox_MainActivity_setSignatureHash(env *C.JNIEnv, clazz C.jclass, hash C.jstring) {
	cstr := C.getJString(env, hash)
	SetApkSignatureHash(C.GoString(cstr))
	C.freeJString(env, hash, cstr)
}

//export Java_org_youyuan_toolbox_MainActivity_setPackageName
func Java_org_youyuan_toolbox_MainActivity_setPackageName(env *C.JNIEnv, clazz C.jclass, pkg C.jstring) {
	cstr := C.getJString(env, pkg)
	SetApkPackageName(C.GoString(cstr))
	C.freeJString(env, pkg, cstr)
}

//export Java_org_youyuan_toolbox_MainActivity_setApkPath
func Java_org_youyuan_toolbox_MainActivity_setApkPath(env *C.JNIEnv, clazz C.jclass, path C.jstring) {
	cstr := C.getJString(env, path)
	SetApkPath(C.GoString(cstr))
	C.freeJString(env, path, cstr)
}

//export Java_org_youyuan_toolbox_MainActivity_GoMain
func Java_org_youyuan_toolbox_MainActivity_GoMain(env *C.JNIEnv, clazz C.jclass) {
	defer func() {
		if r := recover(); r != nil {
			writeStartupLog(fmt.Sprintf("PANIC: %v\n%s", r, debug.Stack()))
		}
	}()

	writeStartupLog("JNI backend entry")
	C.captureVM(env)

	port := "19429"
	writeStartupLog(fmt.Sprintf("starting on 127.0.0.1:%s data=%s", port, appDataDir))

	if err := StartServer(port, appDataDir); err != nil {
		writeStartupLog(fmt.Sprintf("ERROR: %v", err))
	}
}

//export notifyFloating
func notifyFloating(json string) {
	cstr := C.CString(json)
	defer C.free(unsafe.Pointer(cstr))
	C.callFloatingBridge(cstr)
}

// launchAndroidApp 经 JNI 请求 Java 启动指定包名的应用。
func launchAndroidApp(pkg string) {
	cstr := C.CString(pkg)
	defer C.free(unsafe.Pointer(cstr))
	C.callLaunchApp(cstr)
}

// callAndroidSystem 经 JNI 同步调用 Java FloatingBridge.call(method, paramsJSON)，
// 返回 Java 侧 JSON 结果。桌面构建由 system_stub.go 提供空实现。
func callAndroidSystem(method, paramsJSON string) (string, error) {
	m := C.CString(method)
	defer C.free(unsafe.Pointer(m))
	p := C.CString(paramsJSON)
	defer C.free(unsafe.Pointer(p))
	res := C.callJavaSystem(m, p)
	defer C.free(unsafe.Pointer(res))
	return C.GoString(res), nil
}

// Java -> Go 命令通道：JniFloatingChannel.floatInit()。
// 无需特殊初始化（FloatingBridge 类引用已在 captureVM 缓存）。
//
//export Java_org_youyuan_toolbox_JniFloatingChannel_floatInit
func Java_org_youyuan_toolbox_JniFloatingChannel_floatInit(env *C.JNIEnv, clazz C.jclass) {}

//export Java_org_youyuan_toolbox_JniFloatingChannel_floatSend
func Java_org_youyuan_toolbox_JniFloatingChannel_floatSend(env *C.JNIEnv, clazz C.jclass, typ C.jstring, data C.jstring) {
	tC := C.getJString(env, typ)
	dC := C.getJString(env, data)
	t := C.GoString(tC)
	d := C.GoString(dC)
	C.freeJString(env, typ, tC)
	C.freeJString(env, data, dC)

	var v any
	if err := json.Unmarshal([]byte(d), &v); err != nil {
		v = nil
	}
	handleFloatingCommand(t, v)
}

func writeStartupLog(msg string) {
	text := C.CString(msg)
	C.yyStartupLog(text)
	C.free(unsafe.Pointer(text))
	if appDataDir == "" {
		return
	}
	os.WriteFile(appDataDir+"/startup.log", []byte(msg+"\n"), 0644)
}

func main() {}

//export Java_org_youyuan_toolbox_AccountProcessService_startCore
func Java_org_youyuan_toolbox_AccountProcessService_startCore(env *C.JNIEnv, clazz C.jclass, dir, port C.jstring) {
	defer func() {
		if recovered := recover(); recovered != nil {
			writeStartupLog(fmt.Sprintf("PANIC: %v", recovered))
		}
	}()
	directory := C.getJString(env, dir)
	appDataDir = C.GoString(directory)
	C.freeJString(env, dir, directory)
	address := C.getJString(env, port)
	listenPort := C.GoString(address)
	C.freeJString(env, port, address)
	C.captureVM(env)
	SetApkPackageName("org.youyuan.toolbox")
	if err := StartServer(listenPort, appDataDir); err != nil {
		writeStartupLog(err.Error())
	}
}

func yyPythonThreadEnter() error {
	if C.yyAttachWorker() != 0 {
		return fmt.Errorf("JNI 工作线程连接失败")
	}
	return nil
}
func yyPythonThreadLeave() { C.yyDetachWorker() }
