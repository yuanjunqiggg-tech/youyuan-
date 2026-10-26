# 游园工具箱 Android 加固规则（release / R8）
#
# 目标：在不改变任何业务行为的前提下，让 release 包
#   1) 移除未被引用的代码（第三方库占绝大多数体积）；
#   2) 压缩未引用的资源；
#   3) 混淆第三方库的类名/成员名。
#
# 本工程有一层特殊的「名字硬约束」，必须先说清楚，否则一开 R8 就崩：
#   - Go 内核用 JNI 按**类名字符串**查找类：
#       FindClass("org/youyuan/toolbox/FloatingBridge")
#     并按**方法名**取 jmethodID：emit / launchApp / call；
#   - Go 导出的 JNI 符号名形如 Java_org_youyuan_toolbox_MainActivity_setDataDir，
#     JVM 是用「Java 侧声明的类名 + 方法名」去拼这个符号的 —— 改了名字就找不到；
#   - WebView 用 addJavascriptInterface 暴露的对象，JS 按方法名调用。
#
# 所以下面把 App 自己的类名全部保住（-keepnames），只允许成员名和未引用代码被处理。
# 想把 org.youyuan.toolbox.** 也整体改名，需要先在真机上逐项验证 JNI 与 JS 桥，
# 否则就是「混淆成功、功能全挂」。这条留给下一轮做，不在本次范围内赌。

# ── 1. App 自身类名保号 ─────────────────────────────────────────────
# 只保名字，不阻止裁剪/成员混淆；清单里的组件由 AGP 自动加 keep 规则。
-keepnames class org.youyuan.toolbox.**

# 声明了 native 方法的类，其方法名也不能改（JNI 符号按方法名拼）。
-keepclasseswithmembernames,includedescriptorclasses class * {
    native <methods>;
}

# ── 2. Go 内核实际调用的 Java 入口（按方法名 hard-code）────────────
# 与 upstream/libmain.go 里的 GetStaticMethodID 一一对应，改动必须同步。
-keepclassmembers class org.youyuan.toolbox.FloatingBridge {
    public static void emit(java.lang.String);
    public static void launchApp(java.lang.String);
    public static java.lang.String call(java.lang.String, java.lang.String);
}

# ── 3. WebView 的 JS 桥 ────────────────────────────────────────────
# JS 侧 window.android.xxx() / 悬浮窗里的 closeDraw 等，全靠方法名。
-keepclassmembers class * {
    @android.webkit.JavascriptInterface <methods>;
}

# ── 4. 反射与序列化需要的元数据 ───────────────────────────────────
# FloatingBridge 用 getDeclaredMethod 反射 Shizuku；okhttp/JSON 需要泛型签名。
-keepattributes Signature,InnerClasses,EnclosingMethod,*Annotation*

# ── 5. 第三方库 ───────────────────────────────────────────────────
# Shizuku 通过 Binder + 反射调用，整包保留最稳（体积很小，几十 KB）。
-keep class rikka.shizuku.** { *; }
-dontwarn rikka.shizuku.**

# okhttp / okio 自带 consumer 规则，这里只压掉「可选平台实现缺失」的告警，
# 不压的话 R8 会因为缺 conscrypt/bouncycastle 之类直接构建失败。
-dontwarn okhttp3.**
-dontwarn okio.**
-dontwarn org.conscrypt.**
-dontwarn org.bouncycastle.**
-dontwarn org.openjsse.**

# androidx.webkit 在部分 ROM 上会走可选路径。
-dontwarn androidx.webkit.**

# Chaquopy（Python 运行时）的 keep 规则由插件自动注入到
# app/build/python/proguard-rules.pro，这里不要重复声明，避免两边打架。
