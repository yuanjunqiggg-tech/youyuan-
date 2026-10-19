package org.youyuan.toolbox;

import android.app.Activity;
import android.content.Intent;
import android.content.ClipData;
import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.content.pm.Signature;
import android.net.Uri;
import android.os.Build;
import android.provider.Settings;
import androidx.core.content.FileProvider;
import org.json.JSONObject;
import java.io.*;
import java.net.*;
import java.security.MessageDigest;
import java.util.*;
import java.util.concurrent.atomic.AtomicBoolean;

/** Downloads into private cache, verifies identity, then requests an in-place update. */
final class AndroidUpdateManager {
 static final int INSTALL_PERMISSION_CODE=1006;
 interface Listener {void event(JSONObject data);}
 private final Activity activity;
 private final Listener listener;
 private final AtomicBoolean busy=new AtomicBoolean();
 AndroidUpdateManager(Activity activity,Listener listener){this.activity=activity;this.listener=listener;}
 private void emit(String state,String message,long downloaded,long size){try{
  JSONObject event=new JSONObject();event.put("state",state);event.put("message",message);event.put("downloaded",downloaded);event.put("size",size);listener.event(event);
 }catch(Exception ignored){}}
 void start(String address,String hash,long size,int version){
  if(!busy.compareAndSet(false,true)){emit("busy","更新下载正在进行",0,size);return;}
  new Thread(()->{try{
   PackageInfo installed=activity.getPackageManager().getPackageInfo(activity.getPackageName(),0);
   long installedCode=Build.VERSION.SDK_INT>=28?installed.getLongVersionCode():installed.versionCode;
   if(version<=installedCode){emit("up_to_date","当前已是此版本或更新版本，无需重复安装",0,size);return;}
   URL url=new URL(address);
   if(!"https".equals(url.getProtocol())||!"updates.youyuanqi.dpdns.org".equals(url.getHost())||url.getPort()!=-1||url.getUserInfo()!=null||!"/updates/android/latest.apk".equals(url.getPath())||!hash.matches("[a-f0-9]{64}")||size<=0||size>256L*1024*1024)throw new IOException("更新地址或校验信息无效");
   File directory=new File(activity.getCacheDir(),"updates");if(!directory.isDirectory()&&!directory.mkdirs())throw new IOException("无法创建更新缓存");
   File part=new File(directory,hash+".apk"), ready=new File(directory,"pending.apk");
   File legacy=new File(directory,hash+".part");
   if(!part.exists()&&legacy.isFile()&&!legacy.renameTo(part))throw new IOException("无法恢复更新下载");
   // Resume only the artifact with the same expected digest.
   long offset=part.isFile()?part.length():0;if(offset>size){if(!part.delete())throw new IOException("无法重置下载");offset=0;}
   emit("downloading","正在下载更新，账号和插件数据会保留",offset,size);
   if(offset<size){
    java.net.HttpURLConnection connection=(java.net.HttpURLConnection)url.openConnection();
    connection.setInstanceFollowRedirects(false);connection.setConnectTimeout(20000);connection.setReadTimeout(30000);
    if(offset>0){connection.setRequestProperty("Range","bytes="+offset+"-");connection.setRequestProperty("If-Range","\""+hash+"\"");}
    try{
     int status=connection.getResponseCode();
     if(status==200)offset=0;
     else if(status!=206||!String.valueOf(connection.getHeaderField("Content-Range")).equals("bytes "+offset+"-"+(size-1)+"/"+size))throw new IOException("下载服务返回 HTTP "+status);
     long count=offset,last=0;
     try(InputStream input=connection.getInputStream();FileOutputStream output=new FileOutputStream(part,offset>0)){
      byte[] buffer=new byte[65536];int n;
      while((n=input.read(buffer))!=-1){count+=n;if(count>size)throw new IOException("更新文件大小不匹配");output.write(buffer,0,n);long now=System.currentTimeMillis();if(now-last>500){emit("downloading","正在下载更新",count,size);last=now;}}
      output.getFD().sync();
     }
     if(count!=size)throw new IOException("下载未完成，再次更新可续传");
    }finally{connection.disconnect();}
   }
   emit("verifying","校验文件、包名、版本与签名",size,size);
   verify(part,hash,size,version);
   if(ready.exists()&&!ready.delete())throw new IOException("无法替换更新缓存");
   if(!part.renameTo(ready))throw new IOException("无法保存更新包");
   activity.getSharedPreferences("youyuan-update",0).edit().putString("hash",hash).putLong("size",size).putInt("version",version).apply();
   activity.runOnUiThread(this::requestInstall);
  }catch(Exception error){emit("error",error.getMessage()==null?"更新失败":error.getMessage(),0,size);}
  finally{busy.set(false);}},"youyuan-update").start();
 }
 private static String digest(File file)throws Exception{
  MessageDigest hash=MessageDigest.getInstance("SHA-256");try(InputStream input=new FileInputStream(file)){byte[] bytes=new byte[65536];int n;while((n=input.read(bytes))!=-1)hash.update(bytes,0,n);}
  StringBuilder result=new StringBuilder();for(byte value:hash.digest())result.append(String.format(Locale.ROOT,"%02x",value&255));return result.toString();
 }
 private static Set<String> signatures(PackageInfo info)throws Exception{
  Signature[] signatures=Build.VERSION.SDK_INT>=28&&info.signingInfo!=null?info.signingInfo.getApkContentsSigners():info.signatures;
  if(signatures==null||signatures.length==0)throw new IOException("APK 没有有效签名");
  Set<String> result=new HashSet<>();for(Signature signature:signatures)result.add(Base64.getEncoder().encodeToString(MessageDigest.getInstance("SHA-256").digest(signature.toByteArray())));return result;
 }
 private void verify(File file,String hash,long size,int version)throws Exception{
  if(file.length()!=size||!hash.equals(digest(file))) {file.delete();throw new IOException("更新校验失败，请重新下载");}
  int flags=PackageManager.GET_SIGNATURES|(Build.VERSION.SDK_INT>=28?PackageManager.GET_SIGNING_CERTIFICATES:0);
  PackageManager pm=activity.getPackageManager();PackageInfo archive=pm.getPackageArchiveInfo(file.getAbsolutePath(),flags),current=pm.getPackageInfo(activity.getPackageName(),flags);
  if(archive==null||!activity.getPackageName().equals(archive.packageName))throw new IOException("更新包不是游园工具箱");
  long code=Build.VERSION.SDK_INT>=28?archive.getLongVersionCode():archive.versionCode;
  long installed=Build.VERSION.SDK_INT>=28?current.getLongVersionCode():current.versionCode;
  if(code!=version)throw new IOException("更新清单版本与APK实际版本不一致，请刷新更新信息后重试");
  if(code<=installed)throw new IOException("当前已安装相同或更新版本，无需重复安装");
  if(!signatures(archive).equals(signatures(current)))throw new IOException("更新包签名与当前应用不一致");
 }
 void requestInstall(){
  // Re-verify after returning from system settings, or after activity recreation.
  new Thread(()->{try{
   android.content.SharedPreferences prefs=activity.getSharedPreferences("youyuan-update",0);
   File file=new File(activity.getCacheDir(),"updates/pending.apk");
   verify(file,prefs.getString("hash",""),prefs.getLong("size",0),prefs.getInt("version",0));
   activity.runOnUiThread(()->{try{
    if(!activity.getPackageManager().canRequestPackageInstalls()){
     emit("permission_needed","请允许游园安装更新，然后返回应用",file.length(),file.length());
     activity.startActivityForResult(new Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,Uri.parse("package:"+activity.getPackageName())),INSTALL_PERMISSION_CODE);return;
    }
    Uri uri=FileProvider.getUriForFile(activity,activity.getPackageName()+".updates",file);
    Intent intent=new Intent(Intent.ACTION_VIEW).setDataAndType(uri,"application/vnd.android.package-archive").addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
    intent.setClipData(ClipData.newRawUri("游园更新",uri));activity.startActivity(intent);
    emit("installing","请在系统窗口确认覆盖安装，应用数据会保留",file.length(),file.length());
   }catch(Exception error){emit("error",error.getMessage(),0,0);}});
  }catch(Exception error){emit("error",error.getMessage(),0,0);}},"youyuan-install-check").start();
 }
 void permissionResult(){if(activity.getPackageManager().canRequestPackageInstalls())requestInstall();else emit("error","尚未允许安装更新，可在系统设置中允许后重试",0,0);}
}
