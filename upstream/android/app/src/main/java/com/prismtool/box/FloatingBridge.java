package org.youyuan.toolbox;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.os.Build;
import android.os.Handler;
import android.os.Looper;
import android.os.VibrationEffect;
import android.os.Vibrator;

import org.json.JSONObject;

import rikka.shizuku.Shizuku;
import rikka.shizuku.ShizukuRemoteProcess;

/** Go 经 JNI 回调的入口：把一条消息转发给当前活动的 JNI 通道，并对 notify 事件发系统通知。 */
public final class FloatingBridge {
    private static volatile JniFloatingChannel activeChannel;
    private static final Handler main = new Handler(Looper.getMainLooper());
    private static volatile Context appCtx;
    private static volatile com.chaquo.python.PyObject pythonDispatch;

    // ---- Shizuku（ADB/root shell 权限）----
    private static volatile boolean shizukuGranted;
    public static final int SHIZUKU_REQ_CODE = 7001;

    public static synchronized void ensureInitialized(Context ctx) {
        if (appCtx == null) init(ctx);
    }

    public static void init(Context ctx) {
        appCtx = ctx.getApplicationContext();
        try {
            // 注册 Shizuku 授权结果监听，记录是否已授予 ADB 权限
            Shizuku.addRequestPermissionResultListener((requestCode, grantResult) -> {
                if (requestCode == SHIZUKU_REQ_CODE) {
                    shizukuGranted = grantResult == PackageManager.PERMISSION_GRANTED;
                }
            });
        } catch (Throwable ignore) {}
    }
    public static void setActiveChannel(JniFloatingChannel ch) { activeChannel = ch; }

    /** 已授予 Shizuku(ADB) 权限？供 Go 侧查询。 */
    public static boolean isShizukuGranted() {
        try { return Shizuku.checkSelfPermission() == PackageManager.PERMISSION_GRANTED; }
        catch (Throwable t) { return shizukuGranted; }
    }

    /** 由 Go 原生层调用（线程不定）：同步执行一个系统方法，返回 JSON 结果字符串。 */
    public static String call(String method, String paramsJson) {
        if("pythonPlugin".equals(method)) {
            try {
                com.chaquo.python.PyObject dispatch=pythonDispatch;
                if(dispatch==null) synchronized(FloatingBridge.class) {
                    if(pythonDispatch==null) {
                        if(!com.chaquo.python.Python.isStarted())com.chaquo.python.Python.start(new com.chaquo.python.android.AndroidPlatform(appCtx));
                        pythonDispatch=com.chaquo.python.Python.getInstance().getModule("youyuan_plugins").get("dispatch");
                    }
                    dispatch=pythonDispatch;
                }
                return dispatch.call(paramsJson).toString();
            } catch(Throwable error) {
                try{return new JSONObject().put("ok",false).put("error",error.toString()).toString();}catch(Exception ignored){return "{\"ok\":false,\"error\":\"Python 调用失败\"}";}
            }
        }
        JSONObject p = new JSONObject();
        try { if (paramsJson != null && !paramsJson.isEmpty()) p = new JSONObject(paramsJson); } catch (Exception ignore) {}
        try {
            switch (method) {
                case "accountProcess":
                    if(appCtx==null)throw new IllegalStateException("应用上下文尚未初始化");
                    AccountProcessService.control(appCtx,p.optString("action"),p.optInt("slot"),p.optString("id"));
                    return "{\"ok\":true}";
                case "pythonPlugin":
                    synchronized (FloatingBridge.class) {
                        if (!com.chaquo.python.Python.isStarted()) {
                            com.chaquo.python.Python.start(new com.chaquo.python.android.AndroidPlatform(appCtx));
                        }
                    }
                    return com.chaquo.python.Python.getInstance().getModule("youyuan_plugins").callAttr("dispatch", paramsJson).toString();
                case "shell":
                    return runShell(p.optString("command", ""));
                case "shizukuStatus":
                    return shizukuStatusJson();
                case "shizukuRequest":
                    return requestShizukuPermission();
                case "vibrate":
                    vibrate(p.optLong("ms", 500));
                    return "{\"ok\":true}";
                case "notify":
                    postDirectNotification(p.optString("title", "通知"), p.optString("body", ""));
                    return "{\"ok\":true}";
                case "launchApp":
                    launchApp(p.optString("pkg"));
                    return "{\"ok\":true}";
                default:
                    return "{\"ok\":false,\"error\":\"unknown method: " + method + "\"}";
            }
        } catch (Throwable t) {
            return "{\"ok\":false,\"error\":\"" + t.getClass().getSimpleName() + ": " + t.getMessage() + "\"}";
        }
    }

    /** 经 Shizuku 以 shell(ADB) 身份执行命令，返回 {ok, exit, stdout, stderr, uid}。 */
    private static String runShell(String command) {
        if (command == null || command.isEmpty()) return "{\"ok\":false,\"error\":\"empty command\"}";
        try {
            if (!Shizuku.pingBinder()) return "{\"ok\":false,\"error\":\"Shizuku 未运行\"}";
            if (!isShizukuGranted()) return "{\"ok\":false,\"error\":\"未授予 ADB 权限，请调 system.shizukuRequest()\"}";
            ShizukuRemoteProcess proc = newShizukuProcess(new String[]{"/system/bin/sh", "-c", command});
            String stdout = readStream(proc.getInputStream());
            String stderr = readStream(proc.getErrorStream());
            int exit = proc.waitFor();
            return "{\"ok\":true,\"exit\":" + exit
                    + ",\"stdout\":" + JSONObject.quote(stdout)
                    + ",\"stderr\":" + JSONObject.quote(stderr)
                    + ",\"uid\":" + Shizuku.getUid() + "}";
        } catch (Throwable t) {
            return "{\"ok\":false,\"error\":\"" + t.getClass().getSimpleName() + ": " + t.getMessage() + "\"}";
        }
    }

    /** Shizuku.newProcess 是 private，只能经反射调用（官方建议改 UserService，但对简单 shell 够用）。 */
    private static ShizukuRemoteProcess newShizukuProcess(String[] cmdarray) throws Exception {
        java.lang.reflect.Method m = Shizuku.class.getDeclaredMethod(
                "newProcess", String[].class, String[].class, String.class);
        m.setAccessible(true);
        return (ShizukuRemoteProcess) m.invoke(null, cmdarray, null, null);
    }

    private static String readStream(java.io.InputStream is) {
        if (is == null) return "";
        StringBuilder sb = new StringBuilder();
        try (java.io.BufferedReader r = new java.io.BufferedReader(
                new java.io.InputStreamReader(is, java.nio.charset.StandardCharsets.UTF_8))) {
            String line;
            while ((line = r.readLine()) != null) {
                if (sb.length() > 0) sb.append("\n");
                sb.append(line);
            }
        } catch (Exception ignore) {}
        return sb.toString();
    }

    private static String shizukuStatusJson() {
        boolean available = false, granted = false;
        int uid = -1;
        try { available = Shizuku.pingBinder(); } catch (Throwable ignore) {}
        try { granted = isShizukuGranted(); } catch (Throwable ignore) {}
        try { uid = Shizuku.getUid(); } catch (Throwable ignore) {}
        return "{\"available\":" + available + ",\"granted\":" + granted + ",\"uid\":" + uid + "}";
    }

    private static String requestShizukuPermission() {
        try {
            if (!Shizuku.pingBinder()) return "{\"ok\":false,\"error\":\"Shizuku 未运行\"}";
            Shizuku.requestPermission(SHIZUKU_REQ_CODE);
            return "{\"ok\":true,\"requested\":true}";
        } catch (Throwable t) {
            return "{\"ok\":false,\"error\":\"" + t.getClass().getSimpleName() + ": " + t.getMessage() + "\"}";
        }
    }

    /** 由 Go 原生层调用（线程不定），必须切到主线程再分发。 */
    public static void emit(String json) {
        main.post(() -> {
            // notify 事件 → 状态栏系统通知（不依赖悬浮窗是否开启）
            try {
                JSONObject o = new JSONObject(json);
                if ("notify".equals(o.optString("type"))) {
                    JSONObject d = o.optJSONObject("data");
                    if (d != null) postSystemNotification(d);
                }
            } catch (Exception ignore) {}
            JniFloatingChannel ch = activeChannel;
            if (ch != null) ch.deliver(json);
        });
    }

    /** 由 Go 原生层调用：启动指定包名的应用。 */
    public static void launchApp(final String pkg) {
        main.post(() -> {
            Context ctx = appCtx;
            if (ctx == null || pkg == null || pkg.isEmpty()) return;
            try {
                Intent intent = ctx.getPackageManager().getLaunchIntentForPackage(pkg);
                if (intent != null) {
                    intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
                    ctx.startActivity(intent);
                }
            } catch (Exception ignore) {}
        });
    }

    /** 触发一次系统震动（经 Vibrator，内部切主线程）。 */
    private static void vibrate(final long ms) {
        main.post(() -> {
            Context ctx = appCtx;
            if (ctx == null) return;
            try {
                Vibrator v = (Vibrator) ctx.getSystemService(Context.VIBRATOR_SERVICE);
                if (v != null && v.hasVibrator()) {
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                        v.vibrate(VibrationEffect.createOneShot(ms, VibrationEffect.DEFAULT_AMPLITUDE));
                    } else {
                        v.vibrate(ms);
                    }
                }
            } catch (Throwable ignore) {}
        });
    }

    /** 发一条系统状态栏通知（title/body）。 */
    private static void postDirectNotification(String title, String body) {
        JSONObject d = new JSONObject();
        try {
            d.put("title", title);
            d.put("body", body);
            d.put("display_mode", "toast");
            d.put("id", "plugin_" + System.currentTimeMillis());
        } catch (Exception ignore) {}
        postSystemNotification(d);
    }

    private static void postSystemNotification(JSONObject d) {
        Context ctx = appCtx;
        if (ctx == null) return;
        try {
            String title = d.optString("title", "通知");
            String body = d.optString("body", "");
            String mode = d.optString("display_mode", "toast");
            String id = d.optString("id", "notify");

            NotificationManager nm = (NotificationManager) ctx.getSystemService(Context.NOTIFICATION_SERVICE);
            if (nm == null) return;
            String CHANNEL_ID = "prism_push";
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                NotificationChannel ch = new NotificationChannel(CHANNEL_ID, "推送通知", NotificationManager.IMPORTANCE_DEFAULT);
                nm.createNotificationChannel(ch);
            }
            Intent tap = new Intent(ctx, MainActivity.class);
            tap.setFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_CLEAR_TOP);
            PendingIntent pi = PendingIntent.getActivity(ctx, 0, tap, PendingIntent.FLAG_IMMUTABLE);
            Notification.Builder b = new Notification.Builder(ctx, CHANNEL_ID)
                    .setSmallIcon(android.R.drawable.ic_dialog_info)
                    .setContentTitle(title)
                    .setContentText(body)
                    .setContentIntent(pi)
                    .setAutoCancel(true);
            // 一次性/临时通知自动消失；常驻(banner/modal)保留到用户点击
            if (!"banner".equals(mode) && !"modal".equals(mode)) {
                b.setTimeoutAfter(8000);
            }
            nm.notify(id.hashCode(), b.build());
        } catch (Exception ignore) {}
    }
}
