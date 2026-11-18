// SHELL-WIN-ZPRODUCT-1: read-only polling, NOT a rendered-frame/event observer.
// No hooks, input, activation, window writes, or process injection.
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;

public static class Shell409Observer {
    [StructLayout(LayoutKind.Sequential)] struct Rect { public int left,top,right,bottom; }
    [StructLayout(LayoutKind.Sequential)] struct Monitor { public int size; public Rect monitor,work; public uint flags; }
    [StructLayout(LayoutKind.Sequential)] struct Gui { public int size; public uint flags; public IntPtr active,focus,capture,menu,move,caret; public Rect rc; }
    delegate bool EnumProc(IntPtr h,IntPtr p);
    [DllImport("user32.dll")] static extern bool EnumWindows(EnumProc callback,IntPtr p);
    [DllImport("user32.dll",CharSet=CharSet.Unicode)] static extern int GetClassName(IntPtr h,StringBuilder s,int n);
    [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr h,out uint p);
    [DllImport("user32.dll")] static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] static extern bool GetGUIThreadInfo(uint t,ref Gui g);
    [DllImport("user32.dll")] static extern bool GetWindowRect(IntPtr h,out Rect r);
    [DllImport("user32.dll")] static extern bool IsWindowVisible(IntPtr h);
    [DllImport("user32.dll")] static extern bool IsIconic(IntPtr h);
    [DllImport("user32.dll")] static extern bool IsZoomed(IntPtr h);
    [DllImport("user32.dll")] static extern IntPtr GetWindow(IntPtr h,uint cmd);
    [DllImport("user32.dll")] static extern IntPtr GetAncestor(IntPtr h,uint cmd);
    [DllImport("user32.dll")] static extern IntPtr MonitorFromWindow(IntPtr h,uint flags);
    [DllImport("user32.dll",CharSet=CharSet.Unicode)] static extern bool GetMonitorInfo(IntPtr h,ref Monitor m);
    [DllImport("user32.dll")] static extern uint GetDpiForWindow(IntPtr h);
    [DllImport("kernel32.dll")] public static extern ulong GetTickCount64();
    public class Window {
        public long hwnd; public uint pid; public bool visible,iconic,zoomed,below_reference,rect_ok,monitor_ok,target_focus_ok;
        public int[] rect,workarea; public uint dpi,target_focus_pid; public long target_focus_hwnd;
    }
    public class Sample {
        public ulong tick_ms; public long foreground_hwnd,focus_hwnd,reference_hwnd;
        public uint foreground_pid,focus_pid,reference_pid; public bool focus_ok;
        public Window[] engines;
    }
    static uint Owner(IntPtr h) { uint p; GetWindowThreadProcessId(h,out p); return p; }
    static Gui Focus(uint tid) { var g=new Gui(); g.size=Marshal.SizeOf(typeof(Gui)); GetGUIThreadInfo(tid,ref g); return g; }
    static Window Read(IntPtr h,IntPtr reference) {
        var w=new Window(); w.hwnd=h.ToInt64(); w.pid=Owner(h);
        w.visible=IsWindowVisible(h); w.iconic=IsIconic(h); w.zoomed=IsZoomed(h);
        Rect r; w.rect_ok=GetWindowRect(h,out r); w.rect=new[]{r.left,r.top,r.right,r.bottom};
        var m=new Monitor(); m.size=Marshal.SizeOf(typeof(Monitor));
        w.monitor_ok=GetMonitorInfo(MonitorFromWindow(h,2),ref m); w.workarea=new[]{m.work.left,m.work.top,m.work.right,m.work.bottom};
        w.dpi=GetDpiForWindow(h);
        uint ignored; uint tid=GetWindowThreadProcessId(h,out ignored); var g=new Gui(); g.size=Marshal.SizeOf(typeof(Gui));
        w.target_focus_ok=GetGUIThreadInfo(tid,ref g); w.target_focus_hwnd=g.focus.ToInt64(); w.target_focus_pid=Owner(g.focus);
        var above=h;
        for(int i=0;i<4096;i++) { above=GetWindow(above,3); if(above==IntPtr.Zero||above==h)break; if(above==reference){w.below_reference=true;break;} }
        return w;
    }
    public static Sample Capture(long reference) {
        var s=new Sample(); s.tick_ms=GetTickCount64(); s.reference_hwnd=reference; s.reference_pid=Owner(new IntPtr(reference));
        var f=GetForegroundWindow(); s.foreground_hwnd=f.ToInt64(); s.foreground_pid=Owner(f);
        var g=new Gui(); g.size=Marshal.SizeOf(typeof(Gui)); s.focus_ok=GetGUIThreadInfo(0,ref g);
        s.focus_hwnd=g.focus.ToInt64(); s.focus_pid=Owner(g.focus);
        var windows=new List<Window>();
        if(!EnumWindows(delegate(IntPtr h,IntPtr p){
            var cls=new StringBuilder(128); GetClassName(h,cls,128);
            if(cls.ToString()=="Engine"&&GetAncestor(h,2)==h)windows.Add(Read(h,new IntPtr(reference)));
            return true;
        },IntPtr.Zero))throw new Exception("EnumWindows failed");
        s.engines=windows.ToArray(); return s;
    }
    static Thread worker; static volatile bool running; static List<Sample> samples; public static string Error;
    public static void Start(long reference) {
        if(worker!=null)throw new Exception("observer already used; one attempt only");
        samples=new List<Sample>(); samples.Add(Capture(reference)); Error=null; running=true;
        worker=new Thread(delegate(){try {while(running){var s=Capture(reference);lock(samples)samples.Add(s);Thread.Sleep(10);}}catch(Exception e){Error=e.ToString();running=false;}});
        worker.IsBackground=true; worker.Start();
    }
    public static Sample[] Stop() {
        running=false; if(worker!=null&&!worker.Join(5000))throw new Exception("observer stop timeout");
        lock(samples)return samples.ToArray();
    }
}
