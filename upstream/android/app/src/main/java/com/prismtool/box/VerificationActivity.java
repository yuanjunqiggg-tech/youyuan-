package org.youyuan.toolbox;

import android.app.Activity;
import android.os.Bundle;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.TextView;
import android.net.Uri;
import android.content.Intent;

/** A separate, bridge-free browser. Close always returns to the unchanged toolbox. */
public class VerificationActivity extends Activity {
    private WebView browser;
    @Override public void onCreate(Bundle state){
        super.onCreate(state);
        LinearLayout root=new LinearLayout(this);root.setOrientation(LinearLayout.VERTICAL);
        LinearLayout bar=new LinearLayout(this);bar.setPadding(12,8,12,8);bar.setGravity(android.view.Gravity.CENTER_VERTICAL);
        bar.setBackgroundColor(android.graphics.Color.rgb(242,248,246));getWindow().setStatusBarColor(android.graphics.Color.WHITE);getWindow().getDecorView().setSystemUiVisibility(android.view.View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR);
        Button close=new Button(this);close.setText("关闭 · 返回游园");close.setOnClickListener(v->finish());bar.addView(close);
        TextView title=new TextView(this);title.setText("验证服务器");title.setPadding(16,0,0,0);bar.addView(title);
        close.setTextColor(android.graphics.Color.rgb(20,108,94));close.setBackgroundTintList(android.content.res.ColorStateList.valueOf(android.graphics.Color.rgb(220,239,232)));title.setTextColor(android.graphics.Color.rgb(35,60,54));
        root.addView(bar);browser=new WebView(this);browser.getSettings().setJavaScriptEnabled(true);browser.getSettings().setDomStorageEnabled(true);
        browser.setWebViewClient(new WebViewClient(){@Override public boolean shouldOverrideUrlLoading(WebView view,android.webkit.WebResourceRequest request){Uri uri=request.getUrl();if("https".equals(uri.getScheme())||"http".equals(uri.getScheme()))return false;try{startActivity(new Intent(Intent.ACTION_VIEW,uri));}catch(Exception ignored){}return true;}});
        root.addView(browser,new LinearLayout.LayoutParams(-1,0,1));setContentView(root);
        String address=getIntent().getStringExtra("url");if(address!=null&&address.startsWith("https://"))browser.loadUrl(address);
    }
    @Override public void onBackPressed(){finish();}
    @Override protected void onDestroy(){if(browser!=null){browser.stopLoading();browser.destroy();}super.onDestroy();}
}
