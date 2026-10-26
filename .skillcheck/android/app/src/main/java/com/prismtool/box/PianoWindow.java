package org.youyuan.toolbox;

import android.content.Context;
import android.view.View;
import android.widget.TextView;

import org.json.JSONObject;

/**
 * 钢琴收起窗：常驻置顶的小胶囊。点击切换全屏钢琴悬浮窗。
 * 作为打开全屏钢琴的锚点，不响应长按关闭（不可关闭）。
 */
public final class PianoWindow implements FloatingWindow {
    private final Runnable onToggleFull;
    private TextView label;

    public PianoWindow(Runnable onToggleFull) { this.onToggleFull = onToggleFull; }

    @Override public String name() { return "piano"; }

    @Override public View createView(Context ctx) {
        label = new TextView(ctx);
        label.setText("♪ 钢琴");
        label.setTextColor(0xFFFBBF24);
        label.setTextSize(14);
        label.setPadding(16, 10, 16, 10);
        return label;
    }

    @Override public void onMessage(String type, String jsonData) {
        try {
            if ("music_progress".equals(type)) {
                JSONObject o = new JSONObject(jsonData);
                String n = o.optString("current_note", "");
                if (!n.isEmpty()) label.setText("♪ " + n);
            } else if ("music_done".equals(type)) {
                label.setText("♪ 钢琴");
            }
        } catch (Exception ignored) {}
    }
}
