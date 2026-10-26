package org.youyuan.toolbox;

import android.content.Context;
import android.view.View;
import android.widget.Button;
import android.widget.ImageView;
import android.graphics.BitmapFactory;
import android.widget.LinearLayout;
import android.widget.ProgressBar;
import android.widget.TextView;

import org.json.JSONObject;

/** 连接状态窗：连接状态 + TPS + 任务进度（名+条）+ 停止任务按钮。 */
public final class StatusWindow implements FloatingWindow {
    private final CommandSender sender;
    private TextView statusLine;
    private TextView taskName;
    private ProgressBar bar;
    private Button stopBtn;

    public StatusWindow(CommandSender sender) { this.sender = sender; }

    @Override public String name() { return "status"; }

    @Override public View createView(Context ctx) {
        LinearLayout ll = new LinearLayout(ctx);
        ll.setOrientation(LinearLayout.VERTICAL);
        ll.setPadding(8, 4, 8, 4);
        ll.setMinimumWidth(150);
        ImageView portrait = new ImageView(ctx);
        try(java.io.InputStream image = ctx.getAssets().open("floating.jpg")) { portrait.setImageBitmap(BitmapFactory.decodeStream(image)); } catch(Exception ignored) {}
        portrait.setScaleType(ImageView.ScaleType.CENTER_CROP);
        portrait.setContentDescription("游园悬浮窗");
        int size = (int)(64 * ctx.getResources().getDisplayMetrics().density);
        ll.addView(portrait,new LinearLayout.LayoutParams(size,size));

        statusLine = new TextView(ctx);
        statusLine.setTextColor(0xFFE2E8F0);
        statusLine.setTextSize(12);
        statusLine.setText("○ 未连接");
        ll.addView(statusLine);

        taskName = new TextView(ctx);
        taskName.setTextColor(0xFF94A3B8);
        taskName.setTextSize(11);
        taskName.setText("无任务");
        ll.addView(taskName);

        bar = new ProgressBar(ctx, null, android.R.attr.progressBarStyleHorizontal);
        bar.setMax(100);
        ll.addView(bar, new LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT, 12));

        stopBtn = new Button(ctx);
        stopBtn.setText("停止任务");
        stopBtn.setTextSize(11);
        stopBtn.setMinHeight(0);
        stopBtn.setMinimumHeight(0);
        stopBtn.setVisibility(View.GONE); // 只在任务运行时显示
        stopBtn.setOnClickListener(v -> sender.send("task_stop", "{}"));
        ll.addView(stopBtn, new LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.WRAP_CONTENT, LinearLayout.LayoutParams.WRAP_CONTENT));

        return ll;
    }

    @Override public void onMessage(String type, String jsonData) {
        try {
            switch (type) {
                case "status": {
                    JSONObject o = new JSONObject(jsonData);
                    boolean conn = o.optBoolean("connected", false);
                    String server = o.optString("server", "");
                    double tps = o.optDouble("tps", 0);
                    boolean tpsKnown = !o.isNull("tps") && !Double.isNaN(tps) && !Double.isInfinite(tps) && tps >= 0;
                    String tpsTxt = tpsKnown ? String.format("TPS: %.1f", tps) : "TPS: 暂不可用";
                    statusLine.setText((conn ? "● 已连接" : "○ 未连接")
                            + (server.isEmpty() ? "" : "  " + server) + "   " + tpsTxt);
                    break;
                }
                case "progress":
                case "task_progress":
                case "task_start": {
                    JSONObject o = new JSONObject(jsonData);
                    int cur = o.optInt("current", 0), total = o.optInt("total", 0);
                    String title = o.optString("title", "");
                    if (stopBtn != null) stopBtn.setVisibility(View.VISIBLE);
                    if (total <= 0) {
                        taskName.setText(title.isEmpty() ? "任务进行中" : title);
                        bar.setProgress(0);
                    } else {
                        int pct = Math.min(100, cur * 100 / total);
                        bar.setProgress(pct);
                        taskName.setText((title.isEmpty() ? "任务" : title) + "  " + pct + "%");
                    }
                    break;
                }
                case "task_done":
                case "task_error":
                case "task_stop":
                case "clear_done":
                    taskName.setText("无任务");
                    bar.setProgress(0);
                    if (stopBtn != null) stopBtn.setVisibility(View.GONE);
                    break;
                default:
                    break;
            }
        } catch (Exception ignored) {}
    }
}
