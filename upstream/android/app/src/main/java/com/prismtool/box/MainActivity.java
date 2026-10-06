package org.youyuan.toolbox;

import android.Manifest;
import android.app.Activity;
import android.app.DownloadManager;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Environment;
import android.os.Handler;
import android.os.Looper;
import android.provider.Settings;
import android.database.Cursor;
import android.provider.OpenableColumns;
import android.webkit.JavascriptInterface;
import android.webkit.WebChromeClient;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.webkit.DownloadListener;
import android.webkit.URLUtil;
import android.view.View;
import android.widget.Toast;
import rikka.shizuku.Shizuku;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.ByteArrayOutputStream;
import java.util.Base64;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.content.pm.Signature;

public class MainActivity extends Activity {

    private WebView webView;
    private AndroidUpdateManager updateManager;
    private Handler handler = new Handler(Looper.getMainLooper());
    private static final int FILE_PICK_CODE = 1001;
    private static final int HTML_FILE_PICK_CODE = 1004;
    private android.webkit.ValueCallback<Uri[]> htmlFileCallback;
    private static final int STORAGE_PERM_CODE = 1002;
    private static final int EXPORT_FILE_CODE = 1003;
    private String lastPickedPath = "";
    private String pendingExportPath = "";        // SAF「另存为」待写出的私有文件路径
    private String pendingExternalFile = null;   // 外部「打开/分享」待投递给前端的文件路径
    private boolean appReady = false;             // App 页面(Go server)是否已加载就绪
    private static final java.util.concurrent.atomic.AtomicBoolean backendLoopStarted = new java.util.concurrent.atomic.AtomicBoolean(false);

    static {
        System.loadLibrary("youyuan");
    }

    private static native void setDataDir(String dir);
    private static native void setSignatureHash(String hash);
    private static native void setPackageName(String pkg);
    private static native void setApkPath(String path);
    private static native void GoMain();

    static void ensureBackend(android.content.Context context) {
        if (!backendLoopStarted.compareAndSet(false, true)) return;
        android.content.Context app = context.getApplicationContext();
        FloatingBridge.ensureInitialized(app);
        String signature = getApkSignatureHash(app);
        setSignatureHash(signature != null ? signature : "");
        setPackageName(app.getPackageName());
        setApkPath(app.getPackageCodePath());
        String dataDir = app.getFilesDir().getAbsolutePath();
        new Thread(() -> {
            setDataDir(dataDir);
            try {
                while (!Thread.currentThread().isInterrupted()) {
                    try { GoMain(); }
                    catch (Throwable error) { android.util.Log.e("YouYuanBackend", "内置服务异常退出", error); }
                    android.util.Log.w("YouYuanBackend", "内置服务已停止，稍后重新启动");
                    try { Thread.sleep(2000); }
                    catch (InterruptedException stop) { Thread.currentThread().interrupt(); }
                }
            } finally { backendLoopStarted.set(false); }
        }, "youyuan-backend").start();
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        webView = new WebView(this);
        updateManager = new AndroidUpdateManager(this,event -> handler.post(() -> webView.evaluateJavascript("window.yyUpdateNativeEvent && window.yyUpdateNativeEvent("+event.toString()+")",null)));
        WebSettings s = webView.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true);
        s.setAllowFileAccess(true);
        s.setAllowContentAccess(true);
        s.setDatabaseEnabled(true);
        // Reduce GPU usage: no need for smooth scrolling or animated zoom on a tool app
        s.setBuiltInZoomControls(false);
        s.setDisplayZoomControls(false);
        s.setLoadWithOverviewMode(true);
        s.setUseWideViewPort(true);
        // Use software layer initially to avoid GPU compositing
        webView.setLayerType(View.LAYER_TYPE_HARDWARE, null);
        webView.setWebChromeClient(new WebChromeClient() {
            @Override public boolean onShowFileChooser(WebView view,android.webkit.ValueCallback<Uri[]> callback,FileChooserParams params) {
                if(htmlFileCallback!=null)htmlFileCallback.onReceiveValue(null);
                htmlFileCallback=callback;
                Intent pick=new Intent(Intent.ACTION_OPEN_DOCUMENT);pick.addCategory(Intent.CATEGORY_OPENABLE);
                String[] accept=params.getAcceptTypes();String type=accept!=null&&accept.length==1&&!accept[0].isEmpty()&&!accept[0].startsWith(".")?accept[0]:"*/*";
                pick.setType(type);pick.putExtra(Intent.EXTRA_ALLOW_MULTIPLE,params.getMode()==FileChooserParams.MODE_OPEN_MULTIPLE);
                if(accept!=null){java.util.ArrayList<String> valid=new java.util.ArrayList<>();for(String mime:accept)if(mime.contains("/")&&!mime.contains(","))valid.add(mime);if(!valid.isEmpty())pick.putExtra(Intent.EXTRA_MIME_TYPES,valid.toArray(new String[0]));}
                pick.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION|Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION);
                handler.post(()->{try{startActivityForResult(pick,HTML_FILE_PICK_CODE);}catch(Exception error){htmlFileCallback.onReceiveValue(null);htmlFileCallback=null;Toast.makeText(MainActivity.this,"未找到系统文件选择器",Toast.LENGTH_SHORT).show();}});
                return true;
            }
        });
        webView.addJavascriptInterface(new JSBridge(), "android");

        webView.setWebViewClient(new WebViewClient() {
            @Override public boolean shouldOverrideUrlLoading(WebView view,android.webkit.WebResourceRequest request) {
                return ExternalLinks.handle(MainActivity.this,request.getUrl().toString());
            }
            @Override public boolean shouldOverrideUrlLoading(WebView view,String url) {
                return ExternalLinks.handle(MainActivity.this,url);
            }
            @Override
            public void onReceivedError(android.webkit.WebView v, android.webkit.WebResourceRequest req, android.webkit.WebResourceError err) {
                // Will be handled by the retry page's JS
            }
            @Override
            public void onPageFinished(WebView view, String url) {
                // Check if we loaded the actual app (not the loading page)
                if (url.startsWith("http://127.0.0.1:19429")) {
                    // App UI is loaded — 现在可以安全投递外部文件了
                    appReady = true;
                    deliverPendingExternalFile();
                }
            }
        });

        // 建筑市场下载:用系统 DownloadManager 存到系统下载目录,避免 WebView 丢失文件
        webView.setDownloadListener(new DownloadListener() {
            @Override
            public void onDownloadStart(String url, String userAgent, String contentDisposition, String mimetype, long contentLength) {
                try {
                    String filename = guessDownloadFileName(url, contentDisposition, mimetype);
                    DownloadManager.Request req = new DownloadManager.Request(Uri.parse(url));
                    req.addRequestHeader("User-Agent", userAgent);
                    req.setTitle(filename);
                    req.setDescription("游园建筑文件");
                    req.setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED);
                    req.setDestinationInExternalPublicDir(Environment.DIRECTORY_DOWNLOADS, filename);
                    DownloadManager dm = (DownloadManager) getSystemService(DOWNLOAD_SERVICE);
                    if (dm != null) dm.enqueue(req);
                } catch (Exception e) {
                    // 兜底:直接导航下载,至少触发网络请求
                    try { webView.loadUrl(url); } catch (Exception ignored) {}
                }
            }
        });

        setContentView(webView);
        FloatingBridge.init(this); // 供 Go JNI 后台启动应用 / 系统通知使用
        try {
            java.nio.file.Files.write(new File(getFilesDir(), "native-directory").toPath(), getApplicationInfo().nativeLibraryDir.getBytes(java.nio.charset.StandardCharsets.UTF_8));
        } catch (Exception error) { android.util.Log.e("YouYuan", "记录原生插件目录失败", error); }

        // Reserve native system bars; the WebView starts below the status bar.
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) getWindow().setDecorFitsSystemWindows(true);
        else getWindow().clearFlags(android.view.WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS);
        getWindow().setStatusBarColor(android.graphics.Color.WHITE);
        getWindow().setNavigationBarColor(android.graphics.Color.WHITE);
        getWindow().getDecorView().setSystemUiVisibility(View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR);

        // Background service to keep alive
        startService(new Intent(this, KeepService.class));

        // Auto-request permissions on first run
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            if (checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS)
                    != PackageManager.PERMISSION_GRANTED) {
                requestPermissions(new String[]{Manifest.permission.POST_NOTIFICATIONS}, 1003);
            }
        }
        // Write device info to file for Go backend
        try {
            String info = android.os.Build.BRAND + " " + android.os.Build.MODEL + " Android " + android.os.Build.VERSION.RELEASE;
            java.io.FileWriter fw = new java.io.FileWriter(new java.io.File(getFilesDir(), "device_model"));
            fw.write(info);
            fw.close();
        } catch (Exception ignored) {}

        ensureBackend(getApplicationContext());

        // Show loading page that retries until Go server is ready
        // 简化版：Logo 淡入 → 标题打字机 → 副标题淡入，服务就绪后立即跳转
        String retryPage = "<!DOCTYPE html><meta charset=\"UTF-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><style>body{margin:0;background:#23242b url('loading.jpg') center/cover no-repeat;height:100vh;display:flex;align-items:flex-end;font-family:sans-serif;color:white}.box{width:100%;padding:32px;background:linear-gradient(transparent,rgba(15,23,42,.94));box-sizing:border-box}h1{font-size:30px}p{opacity:.85}.bar{height:3px;background:#5eead4;border-radius:4px;animation:pulse 1.5s infinite}@keyframes pulse{50%{opacity:.35}}</style><div class=\"box\"><h1>游园</h1><p>正在启动内置插件环境…</p><div class=\"bar\"></div></div><script>setInterval(()=>fetch('http://127.0.0.1:19429/health').then(r=>{if(r.ok)location.href='http://127.0.0.1:19429/'}).catch(()=>{}),300)</script>";
        webView.loadDataWithBaseURL("file:///android_asset/", retryPage, "text/html", "UTF-8", null);

        // 处理外部「打开方式 / 分享到」传入的文件（启动时）
        handleExternalIntent(getIntent());
    }

    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        setIntent(intent);
        // singleTask 模式下，App 已运行时外部文件走这里
        handleExternalIntent(intent);
    }

    // 接收外部应用的「打开 / 分享」intent，复制文件到 App 私有目录并投递给前端
    private void handleExternalIntent(Intent intent) {
        if (intent == null) return;
        String action = intent.getAction();
        if (action == null) return;
        Uri uri = null;
        if (Intent.ACTION_VIEW.equals(action)) {
            // 文件管理器「打开方式」：文件在 data 里
            uri = intent.getData();
        } else if (Intent.ACTION_SEND.equals(action)) {
            // 分享单个文件：流在 EXTRA_STREAM
            uri = intent.getParcelableExtra(Intent.EXTRA_STREAM);
        } else if (Intent.ACTION_SEND_MULTIPLE.equals(action)) {
            // 分享多个文件：取第一个
            java.util.ArrayList<Uri> uris = intent.getParcelableArrayListExtra(Intent.EXTRA_STREAM);
            if (uris != null && !uris.isEmpty()) uri = uris.get(0);
        }
        if (uri == null) return;
        String path = copyFileFromUri(uri);
        if (path == null || path.isEmpty()) {
            path = uri.toString();
        }
        // 仅接收本 App 支持的建筑/MIDI/插件ZIP 文件；其它类型忽略，避免拦截无关分享
        if (!isSupportedExternalFile(path)) return;
        pendingExternalFile = path;
        deliverPendingExternalFile();
    }

    // 页面就绪后将待处理的外部文件投递给前端 JS；未就绪则等 onPageFinished
    private void deliverPendingExternalFile() {
        if (pendingExternalFile == null || !appReady || webView == null) return;
        final String path = pendingExternalFile;
        pendingExternalFile = null;
        String escaped = path.replace("\\", "\\\\").replace("'", "\\'");
        handler.post(() -> webView.evaluateJavascript(
            "window.__onExternalFile__('" + escaped + "')", null));
    }

    // 本 App 支持的外部文件扩展名（建筑 / MIDI / 插件 ZIP）
    private boolean isSupportedExternalFile(String path) {
        if (path == null) return false;
        String lower = path.toLowerCase();
        return lower.endsWith(".mcstructure")
            || lower.endsWith(".schematic")
            || lower.endsWith(".schem")
            || lower.endsWith(".mcworld")
            || lower.endsWith(".bdx")
            || lower.endsWith(".mid")
            || lower.endsWith(".midi")
            || lower.endsWith(".mmu")
            || lower.endsWith(".zip");
    }

    public class JSBridge {
        @JavascriptInterface public void readClipboardText(){
            handler.post(()->{
                String source=webView.getUrl();
                if(source==null||!source.startsWith("http://127.0.0.1:19429/"))return;
                try{
                    android.content.ClipboardManager manager=(android.content.ClipboardManager)getSystemService(CLIPBOARD_SERVICE);
                    android.content.ClipData clip=manager.getPrimaryClip();
                    CharSequence value=clip!=null&&clip.getItemCount()>0?clip.getItemAt(0).coerceToText(MainActivity.this):"";
                    webView.evaluateJavascript("window.yyAgentClipboardText&&window.yyAgentClipboardText("+org.json.JSONObject.quote(value==null?"":value.toString())+")",null);
                }catch(Exception error){Toast.makeText(MainActivity.this,"请长按粘贴，文件请用上传附件",Toast.LENGTH_SHORT).show();}
            });
        }
        @JavascriptInterface public void installUpdate(String address,String hash,long size,int version){
            handler.post(()->{String source=webView.getUrl();if(source!=null&&source.startsWith("http://127.0.0.1:19429/"))updateManager.start(address,hash,size,version);});
        }
        @JavascriptInterface public void openVerification(String address){
            handler.post(()->{try{Uri uri=Uri.parse(address);if(!"https".equals(uri.getScheme()))throw new IllegalArgumentException("仅支持 HTTPS 验证网站");Intent intent=new Intent(MainActivity.this,VerificationActivity.class);intent.putExtra("url",address);startActivity(intent);}catch(Exception error){Toast.makeText(MainActivity.this,error.getMessage(),Toast.LENGTH_SHORT).show();}});
        }

        @JavascriptInterface
        public void pickFile(String type) {
            // ACTION_GET_CONTENT + createChooser：底部弹出「选择文件来源」面板，
            // 列出文件/相册/下载/云盘等所有能提供该类型文件的 App，供用户挑选
            Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
            intent.addCategory(Intent.CATEGORY_OPENABLE);
            if ("building".equals(type)) {
                // .mcstructure .schematic .bdx .mcworld
                intent.setType("*/*");
                intent.putExtra(Intent.EXTRA_MIME_TYPES, new String[]{
                    "application/octet-stream", "application/x-nbt", "*/*"
                });
            } else if ("image".equals(type)) {
                intent.setType("image/*");
            } else if ("media".equals(type)) {
                intent.setType("*/*");
                intent.putExtra(Intent.EXTRA_MIME_TYPES, new String[]{
                    "application/octet-stream", "audio/*", "*/*"
                });
            } else {
                intent.setType("*/*");
            }
            Intent chooser = Intent.createChooser(intent, "选择文件来源");
            handler.post(()->{try{startActivityForResult(chooser, FILE_PICK_CODE);}catch(Exception e){webView.evaluateJavascript("window.__onFilePicked__('')",null);Toast.makeText(MainActivity.this,"无法打开系统文件选择器",Toast.LENGTH_SHORT).show();}});
        }

        @JavascriptInterface
        public void toast(String msg) {
            handler.post(() -> Toast.makeText(MainActivity.this, msg, Toast.LENGTH_SHORT).show());
        }

        // SAF「另存为」：把私有目录文件直接导出到用户选择的位置（不经过 Download 镜像）。
        @JavascriptInterface
        public void exportFile(String srcPath, String suggestedName) {
            if (srcPath == null || srcPath.isEmpty()) {
                handler.post(() -> Toast.makeText(MainActivity.this, "导出源路径为空", Toast.LENGTH_SHORT).show());
                return;
            }
            pendingExportPath = srcPath;
            String name = suggestedName;
            if (name == null || name.isEmpty()) name = "export.zip";
            Intent intent = new Intent(Intent.ACTION_CREATE_DOCUMENT);
            intent.addCategory(Intent.CATEGORY_OPENABLE);
            intent.setType("*/*");
            intent.putExtra(Intent.EXTRA_TITLE, name);
            try {
                startActivityForResult(intent, EXPORT_FILE_CODE);
            } catch (Exception e) {
                handler.post(() -> Toast.makeText(MainActivity.this, "无法打开保存面板", Toast.LENGTH_SHORT).show());
            }
        }


        @JavascriptInterface
        public void notifyFocus(String title, String msg) {
            KeepService.postFocusNotification(title, msg != null ? msg : "");
        }

        @JavascriptInterface
        public int getStatusBarHeight() {
            int resId = getResources().getIdentifier("status_bar_height", "dimen", "android");
            if (resId > 0) return getResources().getDimensionPixelSize(resId);
            // 兜底：按屏幕密度估算 25dp
            return (int) (25 * getResources().getDisplayMetrics().density);
        }

        @JavascriptInterface
        public void setLightStatusBar(boolean light) {
            handler.post(() -> {
                int flags = getWindow().getDecorView().getSystemUiVisibility();
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
                    if (light) {
                        flags &= ~View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
                    } else {
                        flags |= View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
                    }
                    getWindow().getDecorView().setSystemUiVisibility(flags);
                }
            });
        }

        @JavascriptInterface
        public void setStatusBarColor(String color) {
            handler.post(() -> {
                try {
                    int c = android.graphics.Color.parseColor(color);
                    getWindow().setStatusBarColor(c);
                } catch (Exception ignored) {}
            });
        }

        @JavascriptInterface
        public void openExternal(String url) {
            Intent intent = new Intent(Intent.ACTION_VIEW, Uri.parse(url));
            intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
            startActivity(intent);
        }

        @JavascriptInterface
        public void launchApp(String pkg) {
            FloatingBridge.launchApp(pkg);
        }

        @JavascriptInterface
        public String getLastPickedPath() {
            return lastPickedPath;
        }

        @JavascriptInterface
        public void setEnv(String key, String val) {
            // Pass data dir to JS so it knows where Go stores files
        }

        @JavascriptInterface
        public String getDataDir() {
            return getFilesDir().getAbsolutePath();
        }

        @JavascriptInterface
        public String readFileAsBase64(String path) {
            try {
                InputStream in;
                if (path.startsWith("content://")) {
                    in = getContentResolver().openInputStream(Uri.parse(path));
                } else {
                    in = new FileInputStream(path);
                }
                ByteArrayOutputStream buf = new ByteArrayOutputStream();
                byte[] tmp = new byte[8192];
                int n;
                while ((n = in.read(tmp)) > 0) {
                    buf.write(tmp, 0, n);
                }
                in.close();
                byte[] data = buf.toByteArray();
                // 检测图片格式
                String mime = "image/jpeg";
                if (data.length > 2) {
                    if (data[0] == (byte)0xFF && data[1] == (byte)0xD8) mime = "image/jpeg";
                    else if (data[0] == (byte)0x89 && data[1] == (byte)0x50) mime = "image/png";
                    else if (data[0] == (byte)0x47 && data[1] == (byte)0x49) mime = "image/gif";
                    else if (data[0] == (byte)0x52 && data[1] == (byte)0x49) mime = "image/webp";
                    else if (data[0] == (byte)0x42 && data[1] == (byte)0x4D) mime = "image/bmp";
                }
                String b64 = Base64.getEncoder().encodeToString(data);
                return "data:" + mime + ";base64," + b64;
            } catch (Exception e) {
                return "";
            }
        }

        @JavascriptInterface
        public boolean checkStoragePermission() {
            if (Build.VERSION.SDK_INT < Build.VERSION_CODES.Q) {
                // Android 9 及以下（API < 29）：写公共目录需要 READ + WRITE 同时授予
                return checkSelfPermission(Manifest.permission.READ_EXTERNAL_STORAGE)
                            == PackageManager.PERMISSION_GRANTED
                        && checkSelfPermission(Manifest.permission.WRITE_EXTERNAL_STORAGE)
                            == PackageManager.PERMISSION_GRANTED;
            }
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                return Environment.isExternalStorageManager();
            }
            // Android 10 需要检查 READ_EXTERNAL_STORAGE
            return checkSelfPermission(Manifest.permission.READ_EXTERNAL_STORAGE)
                    == PackageManager.PERMISSION_GRANTED;
        }

        @JavascriptInterface
        public void requestStoragePermission() {
            if (Build.VERSION.SDK_INT < Build.VERSION_CODES.Q) {
                // Android 9 及以下：同时申请 READ + WRITE
                requestPermissions(
                    new String[]{Manifest.permission.READ_EXTERNAL_STORAGE,
                                 Manifest.permission.WRITE_EXTERNAL_STORAGE},
                    STORAGE_PERM_CODE
                );
                return;
            }
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                Intent intent = new Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION);
                intent.setData(Uri.parse("package:" + getPackageName()));
                startActivityForResult(intent, STORAGE_PERM_CODE);
            } else {
                // Android 10 用普通权限请求
                requestPermissions(
                    new String[]{Manifest.permission.READ_EXTERNAL_STORAGE},
                    STORAGE_PERM_CODE
                );
            }
        }

        // 打开/关闭 连接状态悬浮窗（顶部按钮）
        @JavascriptInterface
        public void requestBackgroundProtection() {
            handler.post(() -> {
                android.os.PowerManager power = getSystemService(android.os.PowerManager.class);
                if (power.isIgnoringBatteryOptimizations(getPackageName())) {
                    Toast.makeText(MainActivity.this, "后台电池限制已放行", Toast.LENGTH_SHORT).show();
                    return;
                }
                try { startActivity(new Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS,
                        Uri.parse("package:" + getPackageName()))); }
                catch (RuntimeException error) { startActivity(new Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS)); }
            });
        }

        @JavascriptInterface
        public void toggleFloatingWindow() {
            if (!Settings.canDrawOverlays(MainActivity.this)) {
                Intent i = new Intent(Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
                        Uri.parse("package:" + getPackageName()));
                startActivity(i);
                return;
            }
            Intent s = new Intent(MainActivity.this, FloatingWindowService.class);
            s.putExtra("toggle", "status");
            startService(s);
        }

        // 打开/关闭 钢琴悬浮窗（音乐页按钮）
        @JavascriptInterface
        public void togglePianoWindow() {
            if (!Settings.canDrawOverlays(MainActivity.this)) {
                Intent i = new Intent(Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
                        Uri.parse("package:" + getPackageName()));
                startActivity(i);
                return;
            }
            Intent s = new Intent(MainActivity.this, FloatingWindowService.class);
            s.putExtra("toggle", "piano");
            startService(s);
        }

        // 打开/关闭 画板悬浮窗（画板页按钮）
        @JavascriptInterface
        public void toggleDrawWindow() {
            if (!Settings.canDrawOverlays(MainActivity.this)) {
                Intent i = new Intent(Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
                        Uri.parse("package:" + getPackageName()));
                startActivity(i);
                return;
            }
            Intent s = new Intent(MainActivity.this, FloatingWindowService.class);
            s.putExtra("toggle", "draw");
            startService(s);
        }

    }

    // 把私有文件写到用户选择的 SAF 目标 Uri（另存为）。
    private boolean writeFileToUri(String srcPath, Uri destUri) {
        if (srcPath == null || srcPath.isEmpty()) return false;
        try (java.io.InputStream in = new java.io.FileInputStream(srcPath);
             java.io.OutputStream out = getContentResolver().openOutputStream(destUri, "w")) {
            if (out == null) return false;
            byte[] buf = new byte[65536];
            int n;
            while ((n = in.read(buf)) > 0) out.write(buf, 0, n);
            out.flush();
            return true;
        } catch (Exception e) {
            return false;
        }
    }

    private String copyFileFromUri(Uri uri) {
        try {
            // 获取真实文件名
            String name = null;
            try (Cursor cursor = getContentResolver().query(uri, null, null, null, null)) {
                if (cursor != null && cursor.moveToFirst()) {
                    int idx = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME);
                    if (idx >= 0) name = cursor.getString(idx);
                }
            } catch (Exception e) { /* fallback */ }
            if (name == null || name.isEmpty()) {
                name = "imported_" + System.currentTimeMillis();
                String ext = guessExtension(uri);
                name += ext;
            }
            File outFile = new File(getFilesDir(), name);

            try (InputStream in = getContentResolver().openInputStream(uri);
                 FileOutputStream out = new FileOutputStream(outFile)) {
                byte[] buf = new byte[8192];
                int n;
                while ((n = in.read(buf)) > 0) {
                    out.write(buf, 0, n);
                }
            }
            outFile.setReadable(true);
            return outFile.getAbsolutePath();
        } catch (Exception e) {
            return "";
        }
    }

    private static String getApkSignatureHash(android.content.Context app) {
        try {
            PackageInfo info = app.getPackageManager().getPackageInfo(
                app.getPackageName(), PackageManager.GET_SIGNING_CERTIFICATES
            );
            if (info.signingInfo != null && info.signingInfo.getApkContentsSigners() != null
                && info.signingInfo.getApkContentsSigners().length > 0) {
                Signature sig = info.signingInfo.getApkContentsSigners()[0];
                MessageDigest md = MessageDigest.getInstance("SHA-256");
                byte[] hash = md.digest(sig.toByteArray());
                StringBuilder hex = new StringBuilder();
                for (byte b : hash) {
                    hex.append(String.format("%02x", b));
                }
                return hex.toString();
            }
        } catch (Exception e) {
            // silently fail - signature check will report as unavailable
        }
        return null;
    }

    private String guessExtension(Uri uri) {
        String name = uri.getLastPathSegment();
        if (name == null) return "";
        int dot = name.lastIndexOf('.');
        if (dot >= 0) return name.substring(dot);
        return "";
    }

    // 从 Content-Disposition/URL 推断下载文件名,保证有扩展名,便于识别
    private String guessDownloadFileName(String url, String contentDisposition, String mimetype) {
        String fname = URLUtil.guessFileName(url, contentDisposition == null ? "" : contentDisposition, mimetype);
        if (fname == null) fname = "";
        fname = fname.trim();
        if (fname.isEmpty() || fname.equalsIgnoreCase("download")) {
            try {
                String last = url.substring(url.lastIndexOf('/') + 1);
                if (!last.isEmpty()) fname = last;
            } catch (Exception ignored) {}
        }
        // 去掉非法字符,保留扩展名
        fname = fname.replaceAll("[\\\\/:*?\"<>|\\s]", "_");
        if (!fname.contains(".")) fname = fname + ".mcstructure";
        if (fname.length() > 100) fname = fname.substring(fname.length() - 100);
        return fname;
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if(requestCode==AndroidUpdateManager.INSTALL_PERMISSION_CODE){updateManager.permissionResult();return;}
        if(requestCode==HTML_FILE_PICK_CODE){
            Uri[] results=resultCode==RESULT_OK?WebChromeClient.FileChooserParams.parseResult(resultCode,data):null;
            if(resultCode==RESULT_OK&&data!=null&&data.getClipData()!=null){android.content.ClipData clips=data.getClipData();results=new Uri[clips.getItemCount()];for(int i=0;i<results.length;i++)results[i]=clips.getItemAt(i).getUri();}
            if(results!=null)for(Uri uri:results)try{getContentResolver().takePersistableUriPermission(uri,Intent.FLAG_GRANT_READ_URI_PERMISSION);}catch(Exception ignored){}
            if(htmlFileCallback!=null){htmlFileCallback.onReceiveValue(results);htmlFileCallback=null;}return;
        }
        if (requestCode == FILE_PICK_CODE && resultCode == RESULT_OK && data != null && data.getData() != null) {
            Uri uri = data.getData();
            String realPath = copyFileFromUri(uri);
            // 如果 copyFileFromUri 失败，用 content:// URI 兜底
            if (realPath.isEmpty()) {
                realPath = uri.toString();
            }
            lastPickedPath = realPath;
            String escaped = realPath.replace("\\", "\\\\").replace("'", "\\'");
            handler.post(() -> webView.evaluateJavascript(
                "window.__onFilePicked__('" + escaped + "')", null));
        } else if (requestCode == FILE_PICK_CODE && resultCode == RESULT_CANCELED) {
            // 用户取消选择：回调空串，让 JS 侧 promise 尽快结束（无需超时兜底）
            handler.post(() -> webView.evaluateJavascript(
                "window.__onFilePicked__('')", null));
        } else if (requestCode == EXPORT_FILE_CODE && resultCode == RESULT_OK && data != null && data.getData() != null) {
            boolean ok = writeFileToUri(pendingExportPath, data.getData());
            boolean finalOk = ok;
            handler.post(() -> webView.evaluateJavascript(
                "window.__onExportResult__(" + finalOk + ")", null));
            handler.post(() -> Toast.makeText(MainActivity.this, finalOk ? "导出成功" : "导出失败", Toast.LENGTH_SHORT).show());
        } else if (requestCode == STORAGE_PERM_CODE) {
            // 从系统设置页返回后通知 JS
            boolean granted = new JSBridge().checkStoragePermission();
            handler.post(() -> webView.evaluateJavascript(
                "window.__onStoragePermissionResult__(" + granted + ")", null));
        }
    }

    @Override
    public void onBackPressed() {
        webView.evaluateJavascript("window.__onBackPressed__()", val -> {
            if ("false".equals(val) || "null".equals(val) || "\"\"".equals(val) || "\"exit\"".equals(val)) {
                super.onBackPressed();
            }
        });
    }

    @Override
    public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] grantResults) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults);
        if (requestCode == STORAGE_PERM_CODE && grantResults.length > 0) {
            boolean granted = grantResults[0] == PackageManager.PERMISSION_GRANTED;
            handler.post(() -> webView.evaluateJavascript(
                "window.__onStoragePermissionResult__(" + granted + ")", null));
        }
    }

    @Override
    protected void onPause() {
        super.onPause();
        if (webView != null) webView.onPause();
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (webView != null) webView.onResume();
        // 自动尝试申请 Shizuku(ADB) 权限：Shizuku 运行中且未授权时弹系统授权框
        try {
            if (Shizuku.pingBinder() && Shizuku.checkSelfPermission() != PackageManager.PERMISSION_GRANTED) {
                Shizuku.requestPermission(FloatingBridge.SHIZUKU_REQ_CODE);
            }
        } catch (Throwable ignore) {}
    }

    @Override
    protected void onDestroy() {
        super.onDestroy();
        if (webView != null) webView.destroy();
    }
}
