package main

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// Local interactive acceptance runs the real converter handlers without connecting any game bot.
func TestConverterPreviewServer(t *testing.T) {
	addr := os.Getenv("YY_CONVERTER_PREVIEW_ADDR")
	if addr == "" {
		t.Skip("interactive preview only")
	}
	if addr != "127.0.0.1:18926" {
		t.Fatal("preview must bind its dedicated loopback port")
	}
	yyDataRoot = t.TempDir()
	for name, text := range map[string]string{"commands.txt": "say 内置选择器", "building.txt": "fill 0 0 0 2 0 0 stone", "frame1.txt": "第一帧\n中文🙂"} {
		if err := os.WriteFile(filepath.Join(yyDataRoot, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/files/scan", handleFileScan)
	mux.HandleFunc("/api/files/read", handleFileRead)
	yyConverterRoutes(mux)
	mux.Handle("/css/", http.FileServer(http.Dir("frontend")))
	mux.Handle("/js/", http.FileServer(http.Dir("frontend")))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html;charset=utf-8")
		w.Write([]byte(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/css/base.css"><link rel="stylesheet" href="/css/layout.css"><link rel="stylesheet" href="/css/components.css"><link rel="stylesheet" href="/css/youyuan_design.css"><title>游园转换工坊 · 本地验收</title><body style="padding:20px;max-width:1180px;margin:auto"><main id="preview"></main><script>
window.subPageCache={};window.downloads=[];window.ICONS=new Proxy({},{get:()=>''});
window.openSubPage=(name,html)=>document.getElementById('preview').innerHTML=html;
window.yyIntro=(title,body)=>'<h1>'+title+'</h1><p>'+body+'</p>';
window.yySwitch=(id,label,checked)=>'<label><input id="'+id+'" type="checkbox" '+(checked?'checked':'')+'>'+label+'</label>';
window.T=(text,kind)=>console.log(kind,text);
window.A=async(method,path,body)=>{const r=await fetch(path,{method,headers:{'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined});return r.json()};
window.yyDownloadArtifact=async(blob,name)=>{window.downloads.push({name,size:blob.size});const a=document.createElement('a');a.href=URL.createObjectURL(blob);a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(a.href),2000)};
</script><script src="/js/features/filepicker.js"></script><script src="/js/youyuan_converters.js"></script><script>yyConverterPage();</script></body></html>`))
	})
	t.Log("local preview ready", addr)
	if e := http.ListenAndServe(addr, mux); e != nil {
		t.Fatal(e)
	}
}
