package org.youyuan.toolbox;

import android.app.Service;
import android.content.Intent;
import android.graphics.PixelFormat;
import android.graphics.Point;
import android.os.Build;
import android.os.Handler;
import android.os.IBinder;
import android.os.Looper;
import android.util.Log;
import android.view.Gravity;
import android.view.MotionEvent;
import android.view.View;
import android.view.WindowManager;
import android.webkit.JavascriptInterface;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.widget.LinearLayout;
import android.widget.TextView;

import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * 悬浮窗服务：管理多个独立悬浮窗覆盖层。
 * - 状态窗：连接状态+TPS+任务进度+停止任务按钮，长按弹确认关闭。
 * - 收起钢琴窗：常驻置顶小胶囊，点击切换全屏钢琴，不响应长按关闭。
 * - 全屏钢琴窗：占满屏幕的 WebView（/piano），带明显关闭按钮（HTML 调 window.floating.closeFullPiano()）。
 */
public final class FloatingWindowService extends Service {
    private static final String TAG = "FloatingWindow";
    private static FloatingWindowService instance;

    private WindowManager wm;
    private FloatingChannelManager channels;
    private final Map<String, FloatingWindow> defs = new LinkedHashMap<>();
    private final Map<String, View> views = new HashMap<>();
    private final Map<String, WindowManager.LayoutParams> wparams = new HashMap<>();
    private View confirmView; // 长按确认浮层
    private String confirmFor; // 确认浮层针对的窗口名
    private WebView fullPianoView; // 全屏钢琴 WebView
    private WindowManager.LayoutParams fullPianoParams;
    private WebView drawView; // 画板悬浮窗 WebView（全屏，镂空）
    private WindowManager.LayoutParams drawParams;

    public static FloatingWindowService getInstance() { return instance; }

    private int safeTop(){int id=getResources().getIdentifier("status_bar_height","dimen","android");int bar=id>0?getResources().getDimensionPixelSize(id):0;return bar+(int)(14*getResources().getDisplayMetrics().density);}

    @Override public IBinder onBind(Intent i) { return null; }

    private final FloatingChannel.Listener listener = new FloatingChannel.Listener() {
        @Override public void onMessage(String type, String jsonData) {
            for (FloatingWindow w : defs.values()) w.onMessage(type, jsonData);
        }
        @Override public void onStatus(boolean c) { Log.i(TAG, "channel=" + c); }
    };

    @Override public void onCreate() {
        super.onCreate();
        instance = this;
        wm = (WindowManager) getSystemService(WINDOW_SERVICE);
        channels = new FloatingChannelManager(listener);
        channels.start();
        defs.put("status", new StatusWindow(channels::send));
        defs.put("piano", new PianoWindow(this::toggleFullPiano));
        defs.put("draw", new DrawWindow(this::toggleDrawFull));
    }

    @Override public int onStartCommand(Intent intent, int flags, int startId) {
        String toggle = intent != null ? intent.getStringExtra("toggle") : null;
        if (toggle != null) toggleWindow(toggle);
        return START_STICKY;
    }

    /** 开关全屏画板悬浮窗（锚点点击进入 / 关闭后回到锚点；WebView 全屏，背景镂空透出游戏，横屏三段布局）。 */
    public void toggleDrawFull() {
        if (drawView != null && drawView.isAttachedToWindow()) hideDrawFull();
        else showDrawFull();
    }

    private void showDrawFull() {
        if (drawView != null && drawView.isAttachedToWindow()) return;
        WebView wv = new WebView(this);
        WebSettings s = wv.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true);
        s.setAllowFileAccess(true);
        // 镂空：背景全透明，游戏从画板透出（页面 body 也是 transparent）
        wv.setBackgroundColor(0x00000000);
        // 页内桥：关闭 / 调透明度（镂空透出游戏）
        wv.addJavascriptInterface(new Object() {
            @JavascriptInterface public void closeDraw() {
                new Handler(Looper.getMainLooper()).post(FloatingWindowService.this::hideDrawFull);
            }
            @JavascriptInterface public void setDrawOpacity(final float a) {
                new Handler(Looper.getMainLooper()).post(() -> { if (drawView != null) drawView.setAlpha(a); });
            }
        }, "floating");

        WindowManager.LayoutParams p = new WindowManager.LayoutParams(
                WindowManager.LayoutParams.MATCH_PARENT, WindowManager.LayoutParams.MATCH_PARENT,
                overlayType(), WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE, PixelFormat.TRANSLUCENT);
        p.gravity = Gravity.CENTER;
        wm.addView(wv, p);
        drawView = wv;
        drawParams = p;
        wv.loadUrl("http://127.0.0.1:19429/draw");
    }

    private void hideDrawFull() {
        if (drawView != null && drawView.isAttachedToWindow()) wm.removeView(drawView);
        drawView = null;
        drawParams = null;
    }

    /** 开关切换：显示则隐藏，隐藏则显示。 */
    public void toggleWindow(String name) {
        if (views.containsKey(name)) hideWindow(name);
        else showWindow(name);
    }

    public boolean isVisible(String name) { return views.containsKey(name); }

    private void showWindow(String name) {
        FloatingWindow def = defs.get(name);
        if (def == null || views.containsKey(name)) return;
        View v = def.createView(this);
        int type = overlayType();
        WindowManager.LayoutParams p = new WindowManager.LayoutParams(
                WindowManager.LayoutParams.WRAP_CONTENT, WindowManager.LayoutParams.WRAP_CONTENT,
                type, WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE, PixelFormat.TRANSLUCENT);
        p.gravity = Gravity.TOP | Gravity.START;
        p.x = 24 + views.size() * 30;
        p.y = Math.max(320, safeTop()) + views.size() * 44; // 低于系统状态栏，避免重叠溢出
        v.setBackgroundColor(0xCC111827);

        // 收起钢琴窗/画板窗：常驻置顶、点击切全屏、不响应长按关闭；其余窗长按弹确认关闭
        boolean closable = !"piano".equals(name) && !"draw".equals(name);
        Runnable onTap;
        if ("piano".equals(name)) onTap = this::toggleFullPiano;
        else if ("draw".equals(name)) onTap = this::toggleDrawFull;
        else onTap = null;
        attachGestures(name, v, p, closable, onTap);

        wm.addView(v, p);
        views.put(name, v);
        wparams.put(name, p);
    }

    private void hideWindow(String name) {
        View v = views.remove(name);
        wparams.remove(name);
        if (v != null && v.isAttachedToWindow()) wm.removeView(v);
        if (confirmFor != null && confirmFor.equals(name)) dismissConfirm();
    }

    // 统一手势：自由拖动 + 长按(>=600ms,可关闭时)弹确认关闭 + 点按(onTap)
    private void attachGestures(String name, View v, WindowManager.LayoutParams p,
                                boolean closable, Runnable onTap) {
        Point off = new Point();
        final boolean[] moved = {false};
        final long[] downTime = {0};
        v.setOnTouchListener((vv, ev) -> {
            switch (ev.getActionMasked()) {
                case MotionEvent.ACTION_DOWN:
                    off.x = (int) ev.getRawX() - p.x;
                    off.y = (int) ev.getRawY() - p.y;
                    moved[0] = false;
                    downTime[0] = System.currentTimeMillis();
                    return true;
                case MotionEvent.ACTION_MOVE:
                    if (!moved[0]) {
                        int dx = (int) ev.getRawX() - p.x - off.x;
                        int dy = (int) ev.getRawY() - p.y - off.y;
                        if (Math.abs(dx) > 12 || Math.abs(dy) > 12) moved[0] = true;
                    }
                    if (moved[0]) {
                        p.x = (int) ev.getRawX() - off.x;
                        p.y = Math.max(safeTop(), (int) ev.getRawY() - off.y);
                        wm.updateViewLayout(vv, p);
                    }
                    return true;
                case MotionEvent.ACTION_UP:
                    long dur = System.currentTimeMillis() - downTime[0];
                    if (!moved[0]) {
                        if (closable && dur >= 600) showCloseConfirm(name);
                        else if (onTap != null) onTap.run();
                    }
                    return true;
                default:
                    return true;
            }
        });
    }

    private void showCloseConfirm(String name) {
        dismissConfirm();
        confirmFor = name;

        LinearLayout box = new LinearLayout(this);
        box.setOrientation(LinearLayout.VERTICAL);
        box.setPadding(18, 14, 18, 14);
        box.setBackgroundColor(0xE6111827);

        TextView t = new TextView(this);
        t.setTextColor(0xFFE2E8F0);
        t.setText("是否关闭这个悬浮窗？");
        box.addView(t);

        LinearLayout btns = new LinearLayout(this);
        btns.setOrientation(LinearLayout.HORIZONTAL);
        TextView cancel = new TextView(this);
        cancel.setText("取消");
        cancel.setTextColor(0xFF94A3B8);
        cancel.setPadding(18, 8, 18, 8);
        cancel.setOnClickListener(v -> dismissConfirm());
        TextView ok = new TextView(this);
        ok.setText("关闭");
        ok.setTextColor(0xFFF87171);
        ok.setPadding(18, 8, 18, 8);
        ok.setOnClickListener(v -> { dismissConfirm(); hideWindow(name); });
        btns.addView(cancel);
        btns.addView(ok);
        box.addView(btns);

        WindowManager.LayoutParams p = new WindowManager.LayoutParams(
                WindowManager.LayoutParams.WRAP_CONTENT, WindowManager.LayoutParams.WRAP_CONTENT,
                overlayType(), WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE, PixelFormat.TRANSLUCENT);
        p.gravity = Gravity.CENTER;
        wm.addView(box, p);
        confirmView = box;
    }

    private void dismissConfirm() {
        if (confirmView != null && confirmView.isAttachedToWindow()) wm.removeView(confirmView);
        confirmView = null;
        confirmFor = null;
    }

    // 全屏钢琴悬浮窗：显示则隐藏，隐藏则显示。
    public void toggleFullPiano() {
        if (fullPianoView != null && fullPianoView.isAttachedToWindow()) {
            wm.removeView(fullPianoView);
            fullPianoView = null;
            fullPianoParams = null;
            return;
        }
        WebView wv = new WebView(this);
        WebSettings s = wv.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true);
        s.setAllowFileAccess(true);
        // 透明背景：让游戏画面从钢琴窗透出（页面 body 也是 transparent）
        wv.setBackgroundColor(0x00000000);
        // HTML 里明显的关闭按钮调 window.floating.closeFullPiano()，保证一定能关闭。
        // JS 接口方法在后台线程调用，WindowManager 操作必须切回主线程。
        wv.addJavascriptInterface(new Object() {
            @JavascriptInterface public void closeFullPiano() {
                new Handler(Looper.getMainLooper()).post(FloatingWindowService.this::toggleFullPiano);
            }
        }, "floating");

        WindowManager.LayoutParams p = new WindowManager.LayoutParams(
                WindowManager.LayoutParams.MATCH_PARENT, WindowManager.LayoutParams.MATCH_PARENT,
                overlayType(), WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE, PixelFormat.TRANSLUCENT);
        p.gravity = Gravity.CENTER;
        wm.addView(wv, p);
        fullPianoView = wv;
        fullPianoParams = p;
        wv.loadUrl("http://127.0.0.1:19429/piano");
    }

    private int overlayType() {
        return Build.VERSION.SDK_INT >= Build.VERSION_CODES.O
                ? WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY
                : WindowManager.LayoutParams.TYPE_PHONE;
    }

    @Override public void onDestroy() {
        super.onDestroy();
        if (channels != null) channels.stop();
        for (View v : views.values()) {
            if (v.isAttachedToWindow()) wm.removeView(v);
        }
        views.clear();
        wparams.clear();
        if (confirmView != null && confirmView.isAttachedToWindow()) wm.removeView(confirmView);
        if (fullPianoView != null && fullPianoView.isAttachedToWindow()) wm.removeView(fullPianoView);
        fullPianoView = null;
        if (drawView != null && drawView.isAttachedToWindow()) wm.removeView(drawView);
        drawView = null;
        if (instance == this) instance = null;
    }
}
