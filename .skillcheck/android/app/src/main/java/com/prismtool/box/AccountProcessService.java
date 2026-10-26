package org.youyuan.toolbox;

import android.app.Service;
import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.content.Context;
import android.content.Intent;
import android.os.IBinder;
import android.os.PowerManager;
import java.io.File;
import java.nio.file.Files;
import java.nio.charset.StandardCharsets;

/** Each slot runs in a distinct Android process with its own Go/Python globals. */
public abstract class AccountProcessService extends Service {
    static {System.loadLibrary("youyuan");}
    private static native void startCore(String directory,String port);
    protected abstract int slot();
    private boolean started;
    private PowerManager.WakeLock wake;
    @Override public void onCreate(){
        super.onCreate();
        NotificationManager manager=getSystemService(NotificationManager.class);
        String channel="youyuan_accounts";
        NotificationChannel definition=new NotificationChannel(channel,"账号保活",NotificationManager.IMPORTANCE_LOW);
        definition.setSound(null,null);manager.createNotificationChannel(definition);
        startForeground(100+slot(),new Notification.Builder(this,channel).setSmallIcon(android.R.drawable.ic_menu_manage).setContentTitle("游园账号工作空间").setContentText("独立账号内核运行中").setGroup("youyuan_accounts").setOngoing(true).build());
        FloatingBridge.init(this);
        PowerManager power=getSystemService(PowerManager.class);
        wake=power.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK,"youyuan:account"+slot());wake.setReferenceCounted(false);wake.acquire();
    }
    @Override public int onStartCommand(Intent intent,int flags,int startId){
        if(!started){
            String id=intent==null?null:intent.getStringExtra("id");
            File marker=new File(getFilesDir(),"account-slot-"+slot()+".txt");
            try{
                if(id==null)id=new String(Files.readAllBytes(marker.toPath()),StandardCharsets.UTF_8);
                if(id==null||!id.matches("[A-Za-z0-9_-]+"))throw new IllegalArgumentException("账号 ID 无效");
                Files.write(marker.toPath(),id.getBytes(StandardCharsets.UTF_8));
                File directory=new File(new File(getFilesDir(),"managed-accounts"),id);directory.mkdirs();
                Files.write(new File(directory,"native-directory").toPath(),getApplicationInfo().nativeLibraryDir.getBytes(StandardCharsets.UTF_8));
                Files.write(new File(directory,"device_model").toPath(),("Android / "+android.os.Build.MODEL).getBytes(StandardCharsets.UTF_8));
                final String data=directory.getAbsolutePath();started=true;
                new Thread(()->startCore(data,Integer.toString(19500+slot())),"youyuan-account-core").start();
            }catch(Exception error){stopSelf();return START_NOT_STICKY;}
        }
        return START_STICKY;
    }
    @Override public void onDestroy(){if(wake!=null&&wake.isHeld())wake.release();super.onDestroy();android.os.Process.killProcess(android.os.Process.myPid());}
    @Override public IBinder onBind(Intent intent){return null;}
    public static Class<?> type(int slot){switch(slot){case 1:return Slot1.class;case 2:return Slot2.class;case 3:return Slot3.class;case 4:return Slot4.class;case 5:return Slot5.class;case 6:return Slot6.class;case 7:return Slot7.class;case 8:return Slot8.class;case 9:return Slot9.class;case 10:return Slot10.class;case 11:return Slot11.class;case 12:return Slot12.class;case 13:return Slot13.class;case 14:return Slot14.class;case 15:return Slot15.class;case 16:return Slot16.class;case 17:return Slot17.class;case 18:return Slot18.class;case 19:return Slot19.class;case 20:return Slot20.class;case 21:return Slot21.class;case 22:return Slot22.class;case 23:return Slot23.class;case 24:return Slot24.class;case 25:return Slot25.class;case 26:return Slot26.class;case 27:return Slot27.class;case 28:return Slot28.class;case 29:return Slot29.class;case 30:return Slot30.class;case 31:return Slot31.class;case 32:return Slot32.class;default:throw new IllegalArgumentException("账号槽范围 1–32");}}
    public static void control(Context ctx,String action,int slot,String id){Intent intent=new Intent(ctx,type(slot));if("stop".equals(action)){ctx.stopService(intent);return;}intent.putExtra("id",id);ctx.startForegroundService(intent);}
    public static final class Slot1 extends AccountProcessService{protected int slot(){return 1;}}
    public static final class Slot2 extends AccountProcessService{protected int slot(){return 2;}}
    public static final class Slot3 extends AccountProcessService{protected int slot(){return 3;}}
    public static final class Slot4 extends AccountProcessService{protected int slot(){return 4;}}
    public static final class Slot5 extends AccountProcessService{protected int slot(){return 5;}}
    public static final class Slot6 extends AccountProcessService{protected int slot(){return 6;}}
    public static final class Slot7 extends AccountProcessService{protected int slot(){return 7;}}
    public static final class Slot8 extends AccountProcessService{protected int slot(){return 8;}}
    public static final class Slot9 extends AccountProcessService{protected int slot(){return 9;}}
    public static final class Slot10 extends AccountProcessService{protected int slot(){return 10;}}
    public static final class Slot11 extends AccountProcessService{protected int slot(){return 11;}}
    public static final class Slot12 extends AccountProcessService{protected int slot(){return 12;}}
    public static final class Slot13 extends AccountProcessService{protected int slot(){return 13;}}
    public static final class Slot14 extends AccountProcessService{protected int slot(){return 14;}}
    public static final class Slot15 extends AccountProcessService{protected int slot(){return 15;}}
    public static final class Slot16 extends AccountProcessService{protected int slot(){return 16;}}
    public static final class Slot17 extends AccountProcessService{protected int slot(){return 17;}}
    public static final class Slot18 extends AccountProcessService{protected int slot(){return 18;}}
    public static final class Slot19 extends AccountProcessService{protected int slot(){return 19;}}
    public static final class Slot20 extends AccountProcessService{protected int slot(){return 20;}}
    public static final class Slot21 extends AccountProcessService{protected int slot(){return 21;}}
    public static final class Slot22 extends AccountProcessService{protected int slot(){return 22;}}
    public static final class Slot23 extends AccountProcessService{protected int slot(){return 23;}}
    public static final class Slot24 extends AccountProcessService{protected int slot(){return 24;}}
    public static final class Slot25 extends AccountProcessService{protected int slot(){return 25;}}
    public static final class Slot26 extends AccountProcessService{protected int slot(){return 26;}}
    public static final class Slot27 extends AccountProcessService{protected int slot(){return 27;}}
    public static final class Slot28 extends AccountProcessService{protected int slot(){return 28;}}
    public static final class Slot29 extends AccountProcessService{protected int slot(){return 29;}}
    public static final class Slot30 extends AccountProcessService{protected int slot(){return 30;}}
    public static final class Slot31 extends AccountProcessService{protected int slot(){return 31;}}
    public static final class Slot32 extends AccountProcessService{protected int slot(){return 32;}}
}
