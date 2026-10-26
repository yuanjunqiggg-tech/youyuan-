package org.youyuan.toolbox;

import android.content.Context;
import android.view.View;

/** 悬浮窗窗口：每个窗口是独立的覆盖层，可自由拖动，长按弹确认关闭。 */
public interface FloatingWindow {
    String name();
    View createView(Context ctx);
    void onMessage(String type, String jsonData);
}
