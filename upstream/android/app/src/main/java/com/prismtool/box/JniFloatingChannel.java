package org.youyuan.toolbox;

import android.util.Log;
import org.json.JSONObject;

/** 进程内 JNI 通道：Go 事件经 FloatingBridge.emit 送达，Java 命令经 native 方法发回。 */
public final class JniFloatingChannel implements FloatingChannel {
    private static final String TAG = "JniFloating";
    private Listener listener;
    private volatile boolean running;

    // Java -> Go 命令（由 libprism.so 的 C 导出实现）
    private static native void floatSend(String type, String jsonData);
    private static native void floatInit();

    @Override public void setListener(Listener l) { this.listener = l; }

    @Override public void connect() {
        running = true;
        FloatingBridge.setActiveChannel(this);
        try { floatInit(); } catch (UnsatisfiedLinkError e) { Log.e(TAG, "native not ready", e); }
    }

    @Override public void send(String type, String jsonData) {
        if (!running) return;
        try { floatSend(type, jsonData); } catch (UnsatisfiedLinkError e) { Log.e(TAG, "send failed", e); }
    }

    @Override public void close() {
        running = false;
        FloatingBridge.setActiveChannel(null);
    }

    /** 由 FloatingBridge.emit 在主线程调用。 */
    void deliver(String json) {
        if (!running || listener == null) return;
        try {
            JSONObject o = new JSONObject(json);
            String type = o.optString("type");
            String data = o.optString("data");
            listener.onMessage(type, data);
        } catch (Exception e) {
            Log.e(TAG, "bad message: " + json, e);
        }
    }
}
