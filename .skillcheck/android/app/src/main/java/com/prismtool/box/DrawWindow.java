package org.youyuan.toolbox;

import android.content.Context;
import android.view.View;
import android.widget.TextView;

import org.json.JSONObject;

/**
 * 画板收起窗：常驻置顶的小胶囊。点击切换全屏画板悬浮窗。
 * 作为打开全屏画板的锚点，不响应长按关闭（不可关闭）。
 */
public final class DrawWindow implements FloatingWindow {
    private final Runnable onToggleFull;
    private TextView label;

    public DrawWindow(Runnable onToggleFull) { this.onToggleFull = onToggleFull; }

    @Override public String name() { return "draw"; }

    @Override public View createView(Context ctx) {
        label = new TextView(ctx);
        label.setText("✎ 画板");
        label.setTextColor(0xFFFBBF24);
        label.setTextSize(14);
        label.setPadding(16, 10, 16, 10);
        return label;
    }

    @Override public void onMessage(String type, String jsonData) {
        // 预留：可在此显示当前画笔方块等状态
    }
}
