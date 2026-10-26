(function() {
  var generated = null, busy = false, selectedFiles = {};
  function selectedFile(id) { return selectedFiles[id] || get(id).files[0]; }
  var get = (id) => document.getElementById(id), val = (id) => get(id).value;
  function field(id, label, value, type) {
    return '<label for="' + id + '">' + label + '</label><input id="' + id + '" type="' + (type || "text") + '" value="' + value + '">';
  }
  window.yyConverterPage = function() {
    generated = null; selectedFiles = {};
    delete subPageCache["结构与指令转换"];
    openSubPage("结构与指令转换", yyIntro("转换工坊", "选择文件，确认结果，再下载。转换不需要连接游戏。") + `
    <style>.cv-layout{display:grid;grid-template-columns:minmax(0,1.2fr) minmax(0,1fr);gap:20px;align-items:start}.cv-layout>*{min-width:0}.cv-triple{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}.cv-triple label{display:flex;flex-direction:column;gap:8px}.cv-layout{font-size:15px;line-height:1.65}.cv-layout textarea,.cv-layout input,.cv-layout select{max-width:100%;box-sizing:border-box}.cv-layout textarea{width:100%;min-height:190px}.cv-layout input:not([type=checkbox]),.cv-layout select{width:100%}.cv-layout .yy-readout{font-size:13px}.cv-tabs{display:flex;flex-wrap:wrap;gap:8px;margin:0 0 20px}.cv-preview{white-space:pre-wrap;overflow-wrap:anywhere;min-height:140px;max-height:360px;overflow:auto}.cv-layout details{margin-top:18px}.cv-layout details pre{white-space:pre-wrap;overflow-wrap:anywhere}.cv-layout [hidden]{display:none!important}@media(max-width:760px){.cv-layout{grid-template-columns:minmax(0,1fr)}.cv-tabs button{flex:1 1 42%}.cv-triple{gap:8px}}</style>
    <div class="cv-tabs" role="tablist" aria-label="转换类型">
      <button class="yy-filter active" id="cv-tab-structure" role="tab" aria-selected="true" onclick="yyConversionTab('structure')">建筑文件</button>
      <button class="yy-filter" id="cv-tab-blocks" role="tab" aria-selected="false" onclick="yyConversionTab('blocks')">指令转结构</button>
      <button class="yy-filter" id="cv-tab-animation" role="tab" aria-selected="false" onclick="yyConversionTab('animation')">动画帧</button>
      <button class="yy-filter" id="cv-tab-commands" role="tab" aria-selected="false" onclick="yyConversionTab('commands')">命令文本</button>
    </div><div class="cv-layout"><div class="yy-form-stack">
      <section class="card" id="cv-structure"><h3>建筑文件转换</h3>
        <label for="cv-file">选择文件</label><input type="file" id="cv-file" accept=".mcstructure,.bdx,.schem,.schematic,.json,.txt,.mcfunction">
        <p class="yy-hint">支持 MCStructure、BDX、Schem、Schematic、JSON，以及 setblock / fill 放置指令 TXT。</p>
        <label for="cv-target">保存为</label><select id="cv-target"><option value="mcstructure">MCStructure 建筑</option><option value="bdx">BDX 建筑</option><option value="schem">Schem 建筑</option><option value="txt">TXT 放置指令</option><option value="json">JSON 方块表</option><option value="command_chains">提取命令方块中的指令链</option></select>
        <details><summary>旋转与放置设置</summary><label for="cv-rotation">旋转</label><select id="cv-rotation"><option value="0">不旋转</option><option value="90">90°</option><option value="180">180°</option><option value="270">270°</option></select>
        <label for="cv-origin">放置原点 · X,Y,Z</label><input id="cv-origin" value="0,80,0"><label for="cv-coordinates">TXT 坐标</label><select id="cv-coordinates"><option value="absolute">绝对坐标</option><option value="relative">~ 相对坐标</option></select>
        ${yySwitch('cv-merge','相邻同类横行合并为 fill',true)}${yySwitch('cv-air','包含空气，清除结构内空位',false)}
        <p class="yy-hint">TXT 不保留容器物品等 NBT。MCStructure 保留方块实体；此转换不包含游走实体与第二水层。单文件32MiB，区域最多131072方块。</p></details>
        <div class="yy-actions"><button class="btn-s" data-convert-run onclick="yyConvertStructure(true)">先查看</button><button class="btn" data-convert-run onclick="yyConvertStructure(false)">生成文件</button></div>
      </section>
      <section class="card" id="cv-blocks" hidden><h3>指令转结构</h3><p class="yy-hint">把 TXT / mcfunction 中的指令排成相连的命令方块，导入后启动首块。</p>
        <label for="cv-block-file">选择指令文件</label><input type="file" id="cv-block-file" accept=".txt,.mcfunction" onchange="yyReadBlockFile(this)">
        <label for="cv-block-content">或直接粘贴</label><textarea id="cv-block-content" rows="9" placeholder="say 开始&#10;say 完成"></textarea>
        <div class="cv-triple"><label>每层宽度<input id="cv-block-width" type="number" min="1" max="128" value="16"></label><label>每层深度<input id="cv-block-depth" type="number" min="1" max="128" value="16"></label><label>逐块延迟 · 刻<input id="cv-block-delay" type="number" min="0" max="12000" value="0"></label></div>
        <details><summary>TXT 格式说明</summary><p>普通文件每行一条指令。也可用三位属性设置类型、条件、红石；用单独一行 . 分开独立链。</p><pre>2,0,0 20\ntestfor @a\n0,1,0\nsay 检测成功\n.\n1,0,1\nsay 独立脉冲</pre><p class="yy-hint">0=连锁、1=脉冲、2=循环；后两位为条件与红石。20游戏刻=1秒。无法等价写入结构的设置会提示所在行。</p></details>
        <div class="yy-actions"><button class="btn" data-convert-run onclick="yyMakeCommandStructure(false)">生成命令结构</button></div>
      </section>
      <section class="card" id="cv-animation" hidden><h3>文本动画帧</h3><p class="yy-hint">ZIP 内每个 TXT 是一帧，按帧1、帧2、帧10的顺序播放；也可上传单帧 TXT。</p>
        <label for="cv-frame-file">TXT 帧 / ZIP 帧包</label><input type="file" id="cv-frame-file" accept=".txt,.zip">
        <div class="cv-triple"><label>宽度<input id="cv-frame-width" type="number" min="1" max="128" value="16"></label><label>深度<input id="cv-frame-depth" type="number" min="1" max="128" value="16"></label><label>帧间隔 · 刻<input id="cv-frame-delay" type="number" min="0" max="12000" value="2"></label></div>
        <label for="cv-frame-selector">播放给谁</label><input id="cv-frame-selector" value="@a"><label for="cv-frame-display">显示位置</label><select id="cv-frame-display"><option value="actionbar">快捷栏上方</option><option value="title">屏幕标题</option><option value="subtitle">副标题</option></select>
        <label for="cv-frame-output">保存为</label><select id="cv-frame-output"><option value="mcstructure">命令方块结构 · 保留帧间隔</option><option value="mcfunction">mcfunction · 供其他播放器调度</option></select>
        <div class="yy-actions"><button class="btn" data-convert-run onclick="yyMakeCommandStructure(true)">生成动画文件</button></div>
      </section>
      <section class="card" id="cv-commands-panel" hidden><h3>命令文本转换</h3><label for="cv-command-file">TXT / mcfunction</label><input id="cv-command-file" type="file" accept=".txt,.mcfunction" onchange="yyReadCommandFile(this)"><label for="cv-commands">源指令</label><textarea id="cv-commands" rows="9"></textarea>
        <label for="cv-mode">处理方式</label><select id="cv-mode" onchange="yyConvertMode()"><option value="direct">保留原命令链格式</option><option value="execute">指定执行者</option><option value="scoreboard">添加计分板条件</option></select>
        <div id="cv-execute" hidden>${field('cv-selector','执行者','@a')}</div><div id="cv-scoreboard" hidden>${field('cv-objective','计分板名','n')}${field('cv-entity','实体名','C')}${field('cv-start','起始分数','0','number')}<p class="yy-hint">需自行建立计分板并推进分数。</p></div>
        <label for="cv-output">保存为</label><select id="cv-output"><option value="txt">TXT 命令链</option><option value="mcfunction">mcfunction 纯指令</option></select><div class="yy-actions"><button class="btn" data-convert-run onclick="yyConvertCommands()">生成文本</button></div>
      </section>
    </div><div class="yy-form-stack"><section class="card"><h3>转换结果</h3><pre id="cv-result" class="yy-readout cv-preview" aria-live="polite">选好文件后，点击生成。</pre><label for="cv-preview">文本预览</label><textarea id="cv-preview" rows="8" readonly></textarea><label for="cv-name">文件名</label><input id="cv-name" value="转换结果.mcstructure"><div class="yy-actions"><button class="btn-s" id="cv-copy" disabled onclick="yyCopyConversion()">复制文本</button><button class="btn" id="cv-download" disabled onclick="yyDownloadConversion()">下载文件</button></div></section>
      <details class="card"><summary>保存到工具箱命令链</summary><label for="cv-channel">执行通道</label><select id="cv-channel"><option value="ai">自动化</option><option value="player">玩家</option><option value="console">控制台</option></select>${field('cv-delay','逐条延迟 · 毫秒','0','number')}${field('cv-count','运行轮数 · 0为持续','1','number')}<div class="yy-actions"><button class="btn" id="cv-import" disabled onclick="yyImportConversion()">保存，暂不启动</button><button class="btn-s" onclick="yyChainsPage()">管理命令链</button></div></details>
    </div></div>`);
    document.querySelectorAll('.cv-layout input[type=file]').forEach(input => {
      const button=document.createElement('button');button.type='button';button.className='picker-btn';button.textContent='选择文件';input.hidden=true;input.after(button);
      input.addEventListener('change',()=>{delete selectedFiles[input.id];const f=input.files[0];button.textContent=f?f.name:'选择文件';});
      button.onclick=()=>{
        if(typeof showFilePicker!=='function'){input.click();return;}
        showFilePicker('','all',async path=>{
          try{
            const name=path.split(/[\\/]/).pop(),ext='.'+name.split('.').pop().toLowerCase();
            if(!input.accept.split(',').includes(ext))throw Error('请选择 '+input.accept+' 文件');
            const response=await fetch('/api/files/read?path='+encodeURIComponent(path));if(!response.ok)throw Error('读取失败：HTTP '+response.status);
            if((response.headers.get('Content-Type')||'').includes('application/json')){const error=await response.json();throw Error(error.error||'读取失败');}
            const blob=await response.blob(),limit=/block|command/.test(input.id)?2:32;
            if(blob.size>limit*1024*1024)throw Error('文件超过'+limit+'MiB');
            if(!document.contains(input))return;
            const file=new File([blob],name);selectedFiles[input.id]=file;button.textContent=name;
            if(input.id==='cv-block-file')await yyReadBlockFile({files:[file]});
            if(input.id==='cv-command-file')await yyReadCommandFile({files:[file]});
          }catch(e){T(e.message,'e');}
        });
      };
    });
  };
  window.yyConversionTab = function(tab) {
    ['structure','blocks','animation','commands'].forEach(t => {
      get(t === 'commands' ? 'cv-commands-panel' : 'cv-'+t).hidden=t!==tab;
      get('cv-tab-'+t).classList.toggle('active',t===tab);
      get('cv-tab-'+t).setAttribute('aria-selected',String(t===tab));
    });
  };
  window.yyReadBlockFile = async function(input) {
    const f=input.files[0]; if(!f)return;
    if(f.size>2*1024*1024){T('指令文件超过2MiB','e');return;}
    try{get('cv-block-content').value=await f.text();}catch(e){T(e.message,'e');}
  };
  window.yyMakeCommandStructure = function(animation) {
    return work(async()=>{
      const stem=animation?'cv-frame-':'cv-block-';
      const p={mode:animation?'animation':'commands',width:Number(val(stem+'width')),depth:Number(val(stem+'depth')),delay:Number(val(stem+'delay')),output:animation?val('cv-frame-output'):'mcstructure'};
      if(!Number.isInteger(p.width)||!Number.isInteger(p.depth)||!Number.isInteger(p.delay)||p.width<1||p.depth<1||p.width>128||p.depth>128||p.delay<0||p.delay>12000)throw Error('尺寸需为1..128的整数，延迟为0..12000刻');
      if(animation){
        const f=selectedFile('cv-frame-file');if(!f)throw Error('请选择TXT帧或ZIP帧包');if(f.size>32*1024*1024)throw Error('文件超过32MiB');
        p.filename=f.name;p.selector=val('cv-frame-selector');p.display=val('cv-frame-display');
        p.data=await new Promise((resolve,reject)=>{const r=new FileReader();r.onload=()=>resolve(r.result.split(',')[1]);r.onerror=()=>reject(Error('读取失败'));r.readAsDataURL(f);});
      }else p.content=val('cv-block-content');
      const r=await A('POST','/api/youyuan/convert/command-structure',p);if(!r.ok)throw Error(r.error);output(r);
      if(r.frames)get('cv-result').textContent+='\n动画 '+r.frames+' 帧';
    });
  };
  window.yyConvertMode = function() {
    get("cv-execute").hidden = val("cv-mode") !== "execute";
    get("cv-scoreboard").hidden = val("cv-mode") !== "scoreboard";
  };
  function output(r) {
    generated = r;
    get("cv-result").textContent = (r.size ? "\u5C3A\u5BF8 " + r.size.join(" \xD7 ") + " \xB7 " + r.blocks + " \u4E2A\u975E\u7A7A\u6C14\u65B9\u5757\n\u8C03\u8272\u677F " + r.palette + " \u79CD \xB7 \u65B9\u5757\u5B9E\u4F53 " + r.block_entities + " \u4E2A\n" : "") + (r.commands !== void 0 ? r.commands + " \u6761\u547D\u4EE4\u5DF2\u9884\u89E3\u6790\n" : "") + (r.note || "\u8F6C\u6362\u5B8C\u6210\uFF0C\u5C1A\u672A\u542F\u52A8");
    var text = r.content || "";
    get("cv-preview").value = text.slice(0, 1e5);
    get("cv-preview").hidden=!text; document.querySelector('label[for="cv-preview"]').hidden=!text;
    if (text.length > 1e5) get("cv-result").textContent += "\n\u9884\u89C8\u663E\u793A\u524D10\u4E07\u5B57\u7B26\uFF0C\u590D\u5236\u4E0E\u5BFC\u51FA\u4ECD\u4E3A\u5B8C\u6574\u6587\u4EF6\u3002";
    get("cv-name").value = r.filename || "\u8F6C\u6362\u7ED3\u679C.txt";
    get("cv-copy").disabled = !text;
    get("cv-download").disabled = !r.filename;
    get("cv-import").disabled = !r.files && (!text || !["txt", "mcfunction"].includes(r.filename.split(".").pop()));
    if (r.files) get("cv-result").textContent += "\n\u62C6\u5206 " + r.files.length + " \u4E2A\u72EC\u7ACB\u6587\u4EF6\uFF1B\u9ED8\u8BA4\u505C\u7528\u3002\n" + (r.warnings || []).join("\n");
  }
  async function work(fn) {
    if (busy) return;
    busy = true;
    generated=null;["cv-copy","cv-download","cv-import"].forEach(id=>get(id).disabled=true);
    document.querySelectorAll("[data-convert-run]").forEach(b=>b.disabled=true);
    try {
      get("cv-result").textContent = "\u6B63\u5728\u8F6C\u6362\u2026";
      await fn();
    } catch (e) {
      get("cv-result").textContent = "\u8F6C\u6362\u5931\u8D25\uFF1A" + e.message;
      T(e.message, "e");
    } finally {
      busy = false;
      document.querySelectorAll("[data-convert-run]").forEach(b=>b.disabled=false);
    }
  }
  window.yyReadCommandFile = async function(input) {
    try {
      var f = input.files[0];
      if (!f) return;
      if (f.size > 2 * 1024 * 1024) throw Error("\u6307\u4EE4\u6587\u4EF6\u8D85\u8FC72MiB");
      get("cv-commands").value = await f.text();
    } catch (e) {
      T(e.message, "e");
    }
  };
  window.yyConvertStructure = function(inspect) {
    return work(async () => {
      var f = selectedFile("cv-file");
      if (!f) throw Error("\u8BF7\u9009\u62E9\u7ED3\u6784\u6587\u4EF6");
      if (f.size > 32 * 1024 * 1024) throw Error("\u6587\u4EF6\u8D85\u8FC732MiB");
      var origin = val("cv-origin").split(/[ ,，]+/).map(Number);
      if (origin.length !== 3 || origin.some((n) => !Number.isSafeInteger(n) || Math.abs(n) > 3e7)) throw Error("\u539F\u70B9\u9700\u8981\u4E09\u4E2A\u6709\u6548\u6574\u6570");
      var data = await new Promise((resolve, reject) => {
        var reader = new FileReader();
        reader.onload = () => resolve(reader.result.split(",")[1]);
        reader.onerror = () => reject(Error("\u8BFB\u53D6\u6587\u4EF6\u5931\u8D25"));
        reader.readAsDataURL(f);
      });
      var r = await A("POST", "/api/youyuan/convert/structure", { filename: f.name, data, target: inspect ? "inspect" : val("cv-target"), origin, rotation: Number(val("cv-rotation")), relative: val("cv-coordinates") === "relative", merge: get("cv-merge").checked, includeAir: get("cv-air").checked });
      if (!r.ok) throw Error(r.error);
      if (inspect) {
        get("cv-result").textContent = "\u5C3A\u5BF8 " + r.size.join(" \xD7 ") + "\n\u975E\u7A7A\u6C14\u65B9\u5757 " + r.blocks + " \xB7 \u8C03\u8272\u677F " + r.palette + " \xB7 \u65B9\u5757\u5B9E\u4F53 " + r.block_entities + "\n" + r.note;
      } else output(r);
    });
  };
  window.yyConvertCommands = function() {
    return work(async () => {
      var r = await A("POST", "/api/youyuan/convert/commands", { content: val("cv-commands"), mode: val("cv-mode"), objective: val("cv-objective"), entity: val("cv-entity"), selector: val("cv-selector"), start: Number(val("cv-start")), output: val("cv-output") });
      if (!r.ok) throw Error(r.error);
      r.filename = "\u6267\u884C\u547D\u4EE4." + val("cv-output");
      output(r);
    });
  };
  window.yyCopyConversion = async function() {
    if (!(generated == null ? void 0 : generated.content)) return;
    try {
      if (window.android && typeof android.copyToClipboard === "function") android.copyToClipboard(generated.content);
      else await navigator.clipboard.writeText(generated.content);
      T("\u5B8C\u6574\u6587\u672C\u5DF2\u590D\u5236", "s");
    } catch (e) {
      T("\u590D\u5236\u5931\u8D25\uFF1A" + e.message, "e");
    }
  };
  window.yyDownloadConversion = async function() {
    if (!generated) return;
    try {
      var blob = generated.data ? new Blob([Uint8Array.from(atob(generated.data), (c) => c.charCodeAt(0))]) : new Blob([generated.content], { type: "text/plain;charset=utf-8" });
      await yyDownloadArtifact(blob, val("cv-name") || generated.filename);
    } catch (e) {
      T(e.message, "e");
    }
  };
  window.yyImportConversion = async function() {
    if (!(generated == null ? void 0 : generated.content)) return;
    try {
      if (generated.files) {
        await yyImportChainBundle();
        return;
      }
      var name = val("cv-name");
      var rows = await A("GET", "/api/youyuan/chains");
      if (!rows.ok) throw Error(rows.error);
      if ((rows.files || []).some((f) => {
        var _a;
        return f.name === name || ((_a = f.config) == null ? void 0 : _a.name) === name;
      })) throw Error("\u540C\u540D\u547D\u4EE4\u94FE\u5DF2\u5B58\u5728\uFF0C\u8BF7\u4FEE\u6539\u6587\u4EF6\u540D");
      var delay = Number(val("cv-delay")), count = Number(val("cv-count"));
      if (!Number.isInteger(delay) || delay < 0 || !Number.isInteger(count) || count < 0) throw Error("\u5EF6\u8FDF\u4E0E\u8F6E\u6570\u9700\u8981\u975E\u8D1F\u6574\u6570");
      var r = await A("POST", "/api/youyuan/chains", { action: "save", name, content: generated.content, config: { enabled: false, mode: "fast", interval_ms: 1e3, delay_ms: delay, run_count: count, channel: val("cv-channel") } });
      if (!r.ok) throw Error(r.error);
      get("cv-result").textContent += "\n\u5DF2\u4FDD\u5B58\u4E3A\u672A\u542F\u52A8\u7684\u547D\u4EE4\u94FE\uFF1A" + name;
      T("\u5DF2\u5BFC\u5165\uFF0C\u5C1A\u672A\u542F\u52A8", "s");
    } catch (e) {
      T(e.message, "e");
    }
  };
  async function yyImportChainBundle() {
    var r = await A("GET", "/api/youyuan/chains");
    if (!r.ok) throw Error(r.error);
    var existing = new Set((r.files || []).map((f2) => {
      var _a;
      return ((_a = f2.config) == null ? void 0 : _a.name) || f2.name;
    }));
    for (var f of generated.files) if (existing.has(f.name)) throw Error("\u540C\u540D\u6587\u4EF6\u5DF2\u5B58\u5728\uFF1A" + f.name + "\uFF1B\u8BF7\u4FEE\u6539\u6E90\u6587\u4EF6\u540D\u540E\u91CD\u65B0\u63D0\u53D6");
    var created = [];
    try {
      for (var f of generated.files) {
        var result = await A("POST", "/api/youyuan/chains", { action: "save", name: f.name, content: f.content, config: { enabled: false, mode: "fast", channel: val("cv-channel"), interval_ms: 50, delay_ms: 0, run_count: 1 } });
        if (!result.ok) throw Error(result.error);
        created.push(f.name);
      }
    } catch (e) {
      var left = [];
      for (var n of created) {
        try {
          var removed = await A("POST", "/api/youyuan/chains", { action: "delete", name: n });
          if (!removed.ok) left.push(n);
        } catch (_) {
          left.push(n);
        }
      }
      throw Error(e.message + (left.length ? "\uFF1B\u672A\u56DE\u6EDA\u6587\u4EF6\uFF1A" + left.join("\u3001") : ""));
    }
    get("cv-result").textContent += "\n\u5DF2\u4FDD\u5B58 " + created.length + " \u6761\u672A\u542F\u52A8\u547D\u4EE4\u94FE\uFF0C\u9ED8\u8BA4\u5355\u6B21\uFF1B\u5BA1\u6838\u7EA2\u77F3\u3001\u52A8\u6001\u63A5\u7EBF\u4E0E\u539F\u751F\u5EF6\u8FDF\u540E\u518D\u542F\u52A8\u3002";
    T("\u5DF2\u4FDD\u5B58\u62C6\u5206\u94FE\uFF0C\u5C1A\u672A\u8FD0\u884C", "s");
  }
})();
