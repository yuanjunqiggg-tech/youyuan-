package org.youyuan.toolbox;

import android.os.Handler;
import android.os.Looper;
import android.util.Log;

import org.json.JSONObject;

import java.util.concurrent.TimeUnit;

import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;
import okhttp3.WebSocket;
import okhttp3.WebSocketListener;

/**
 * WebSocket 通道：连 127.0.0.1:19429/ws，自动断线重连（约 3s）。
 *
 * <p>传输实现选择 OkHttp 的官方 WebSocket 客户端（而非手写 HttpURLConnection 帧解析），
 * 由 OkHttp 处理 RFC 6455 握手与帧编解码，避免手写实现的脆弱性。帧内容为 JSON 信封：
 * {@code {"type": "...", "data": "..."}}，解析后经主线程回调 {@link Listener}。
 */
public final class WsFloatingChannel implements FloatingChannel {
    private static final String TAG = "WsFloating";
    private static final String URL = "ws://127.0.0.1:19429/ws";
    private static final long RETRY_DELAY_MS = 3000;

    private final Handler main = new Handler(Looper.getMainLooper());
    private final OkHttpClient client;
    private volatile Listener listener;
    private volatile boolean running;
    private volatile boolean connected;
    private volatile WebSocket webSocket;

    public WsFloatingChannel() {
        client = new OkHttpClient.Builder()
                .connectTimeout(2, TimeUnit.SECONDS)
                .readTimeout(0, TimeUnit.MILLISECONDS) // 长连接不设读超时
                .pingInterval(20, TimeUnit.SECONDS)
                .build();
    }

    @Override public void setListener(Listener l) { this.listener = l; }

    @Override public void connect() {
        running = true;
        open();
    }

    private void open() {
        if (!running) return;
        Request req = new Request.Builder().url(URL).build();
        webSocket = client.newWebSocket(req, new ListenerImpl());
    }

    @Override public void send(String type, String jsonData) {
        WebSocket ws = webSocket;
        if (ws == null || !connected) return;
        try {
            JSONObject o = new JSONObject();
            o.put("type", type);
            o.put("data", jsonData);
            ws.send(o.toString());
        } catch (Exception e) {
            Log.w(TAG, "send failed", e);
        }
    }

    @Override public void close() {
        running = false;
        WebSocket ws = webSocket;
        if (ws != null) ws.close(1000, "close");
        setConnected(false);
    }

    private void scheduleReconnect() {
        if (!running) return;
        main.postDelayed(this::open, RETRY_DELAY_MS);
    }

    private void setConnected(boolean c) {
        if (connected == c) return;
        connected = c;
        Listener l = listener;
        if (l != null) main.post(() -> l.onStatus(c));
    }

    private final class ListenerImpl extends WebSocketListener {
        @Override public void onOpen(WebSocket ws, Response response) {
            webSocket = ws;
            setConnected(true);
        }

        @Override public void onMessage(WebSocket ws, String text) {
            String type = "";
            String data = "";
            try {
                JSONObject o = new JSONObject(text);
                type = o.optString("type");
                data = o.optString("data");
            } catch (Exception e) {
                Log.w(TAG, "bad msg: " + text, e);
            }
            final String t = type;
            final String d = data;
            Listener l = listener;
            if (l != null) main.post(() -> l.onMessage(t, d));
        }

        @Override public void onFailure(WebSocket ws, Throwable t, Response response) {
            Log.w(TAG, "ws failure: " + t);
            setConnected(false);
            scheduleReconnect();
        }

        @Override public void onClosed(WebSocket ws, int code, String reason) {
            setConnected(false);
            scheduleReconnect();
        }
    }
}
