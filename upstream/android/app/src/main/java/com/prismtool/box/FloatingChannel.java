package org.youyuan.toolbox;

/** 悬浮窗通信通道抽象。WebSocket 与 JNI 两种实现，对上层透明。 */
public interface FloatingChannel {
    interface Listener {
        void onMessage(String type, String jsonData);
        void onStatus(boolean connected);
    }
    void setListener(Listener l);
    void connect();
    void send(String type, String jsonData);
    void close();
}
