package org.youyuan.toolbox;

/** 悬浮窗向 Go 发送命令的通道抽象（兼容 WS 与 JNI 两条链路）。 */
public interface CommandSender {
    void send(String type, String jsonData);
}
