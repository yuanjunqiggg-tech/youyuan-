package org.youyuan.toolbox;

import android.app.Activity;
import android.app.AlertDialog;
import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.Intent;
import android.net.Uri;

/** Keep registration links out of WebView's unsupported-scheme error page. */
final class ExternalLinks {
    static boolean handle(Activity activity, String address) {
        Uri uri=Uri.parse(address);String scheme=uri.getScheme();
        if(scheme==null||scheme.equals("https")||scheme.equals("http")||scheme.equals("about")||scheme.equals("data")||scheme.equals("blob"))return false;
        Intent intent;String fallback=address;
        try {
            if(scheme.equals("sms")||scheme.equals("smsto")) {
                String raw=uri.getEncodedSchemeSpecificPart();int q=raw.indexOf('?');
                String number=Uri.decode(q<0?raw:raw.substring(0,q));
                String body=q<0?"":Uri.parse("https://local.invalid/?"+raw.substring(q+1)).getQueryParameter("body");
                if(body==null)body="";
                intent=new Intent(Intent.ACTION_SENDTO,Uri.fromParts("smsto",number,null));
                intent.putExtra("sms_body",body);intent.putExtra("body",body);
                fallback="收件号码："+number+"\n短信内容："+body+"\n请在系统短信应用中确认后发送。";
            } else if(scheme.equals("mailto"))intent=new Intent(Intent.ACTION_SENDTO,uri);
            else if(scheme.equals("tel"))intent=new Intent(Intent.ACTION_DIAL,uri);
            else if(scheme.equals("intent")) {intent=Intent.parseUri(address,Intent.URI_INTENT_SCHEME);intent.addCategory(Intent.CATEGORY_BROWSABLE);intent.setComponent(null);intent.setSelector(null);}
            else {showFallback(activity,"未找到对应应用",fallback);return true;}
            activity.startActivity(intent);
        } catch(Exception unavailable) {showFallback(activity,"无法打开系统应用",fallback);}
        return true;
    }
    private static void showFallback(Activity a,String title,String content){
        new AlertDialog.Builder(a).setTitle(title).setMessage(content)
            .setPositiveButton("复制内容",(d,w)->{ClipboardManager c=(ClipboardManager)a.getSystemService(Activity.CLIPBOARD_SERVICE);if(c!=null)c.setPrimaryClip(ClipData.newPlainText("注册信息",content));})
            .setNegativeButton("返回继续",(d,w)->{}).show();
    }
}
