package org.youyuan.toolbox;

import android.os.Handler;
import android.os.Looper;

/** 选择并维护当前悬浮窗通道：优先 WebSocket，失败自动回退 JNI（进程内）。 */
public final class FloatingChannelManager {
    private final WsFloatingChannel ws = new WsFloatingChannel();
    private final JniFloatingChannel jni = new JniFloatingChannel();
    private final FloatingChannel.Listener forward;
    private final Handler main = new Handler(Looper.getMainLooper());
    private volatile FloatingChannel active;
    private boolean jniStarted;

    public FloatingChannelManager(FloatingChannel.Listener forward) { this.forward = forward; }

    public void start() {
        ws.setListener(wsListener);
        ws.connect();
    }

    private final FloatingChannel.Listener wsListener = new FloatingChannel.Listener() {
        @Override public void onMessage(String type, String jsonData) {
            if (active == ws) forward.onMessage(type, jsonData);
        }
        @Override public void onStatus(boolean connected) {
            if (connected) {
                if (active != ws) { active = ws; main.post(() -> forward.onStatus(true)); }
            } else {
                // WS 掉线/连不上 -> 回退 JNI
                if (active == ws || active == null) {
                    if (!jniStarted) { jniStarted = true; jni.setListener(jniListener); jni.connect(); }
                    active = jni;
                }
            }
        }
    };

    private final FloatingChannel.Listener jniListener = new FloatingChannel.Listener() {
        @Override public void onMessage(String type, String jsonData) {
            if (active == jni) forward.onMessage(type, jsonData);
        }
        @Override public void onStatus(boolean connected) {
            if (active == jni) forward.onStatus(connected);
        }
    };

    public void send(String type, String jsonData) {
        FloatingChannel ch = active;
        if (ch != null) ch.send(type, jsonData);
    }

    public void stop() {
        ws.close();
        jni.close();
        active = null;
    }
}
