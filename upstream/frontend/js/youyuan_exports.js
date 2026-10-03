// Show the address actually derived from the selected relay. A custom relay
// must implement the same public routes; a configured address alone is not proof.
var yyRemoteWithAccountState = yyRemotePage;
yyRemotePage = async function () {
  await yyRemoteWithAccountState();
  var state = await yyRootAPI('GET', '/api/youyuan/remote');
  var urls = state.public_urls || {};
  var address = document.querySelector('#yy-local-key')?.closest('.card')?.querySelector('.yy-code');
  if (address) address.textContent = urls.mcp || '请先配置兼容中继';
  var grid = document.querySelector('.yy-form-grid');
  if (!grid) return;
  var card = document.createElement('section');
  card.className = 'card';
  var title = document.createElement('h3');
  title.textContent = '当前公网地址';
  card.appendChild(title);
  var hint = document.createElement('p');
  hint.className = 'yy-hint';
  hint.textContent = state.relay_connected ? '当前设备已接入中继。' : '这是根据配置生成的候选地址；连接成功后才能使用。';
  card.appendChild(hint);
  [['console', '控制台'], ['mcp', 'MCP'], ['api', 'API 目录']].forEach(function (item) {
    var label = document.createElement('label');
    label.textContent = item[1];
    var input = document.createElement('input');
    input.readOnly = true;
    input.value = urls[item[0]] || '';
    var copy = document.createElement('button');
    copy.className = 'btn-s';
    copy.textContent = '复制';
    copy.onclick = function () { navigator.clipboard.writeText(input.value); };
    card.append(label, input, copy);
  });
  var note = document.createElement('p');
  note.className = 'yy-hint';
  note.textContent = '自建中继需提供 /device/connect、/mcp/ 与 API 路由。也可使用下方 APK 内置的临时 MCP 隧道，地址由第三方隧道服务分配。';
  card.appendChild(note);
  grid.appendChild(card);
};

var yyAccountsWithExport = Raccounts;
Raccounts = async function () {
  await yyAccountsWithExport();
  var heading = document.querySelector('.yy-monitor .yy-section-head');
  if (!heading) return;
  var button = document.createElement('button');
  button.className = 'btn-s';
  button.textContent = '导出本账号记录';
  button.onclick = yyExportObservations;
  heading.appendChild(button);
};

async function yyExportObservations() {
  var path = yyMonitorID === 'main' ? '/api/youyuan/observations' : yyAccountPath('/api/youyuan/observations', yyMonitorID);
  if (window.android && android.exportFile) {
    var result = await yyRootAPI('POST', path + '/export', {});
    if (!result.ok) { T(result.error || '导出失败', 'e'); return; }
    android.exportFile(result.src_path, yyMonitorID + '-observations.json');
    return;
  }
  var result = await yyRootAPI('GET', path);
  if (!result.ok) { T(result.error || '读取记录失败', 'e'); return; }
  var blob = new Blob([JSON.stringify({format:'youyuan.observations',version:1,exported_at:new Date().toISOString(),chat:result.chat||[],samples:result.samples||[]},null,2)], {type:'application/json'});
  var url = URL.createObjectURL(blob);
  var link = document.createElement('a');
  link.href = url;
  link.download = yyMonitorID + '-observations.json';
  link.click();
  setTimeout(function () { URL.revokeObjectURL(url); }, 60000);
}

var yyPluginsWithRuntimeExport = Rplg;
Rplg = async function () {
  await yyPluginsWithRuntimeExport();
  var host = document.querySelector('#tab-tasks .content-inner');
  if (!host) return;
  if (host.querySelector('.yy-installed-runtime')) return;
  var result = await A('GET', '/api/youyuan/runtime/list');
  var installed = result.installed || [];
  if (!installed.length) return;
  var card = document.createElement('section');
  card.className = 'card';
  var heading = document.createElement('h3');
  heading.textContent = 'Python / C++ 插件目录导出';
  card.appendChild(heading);
  installed.forEach(function (plugin) {
    var row = document.createElement('div');
    row.className = 'yy-runtime-row';
    var name = document.createElement('span');
    name.textContent = (plugin.name || plugin.id) + ' · ' + plugin.language;
    var button = document.createElement('button');
    button.className = 'btn-s';
    button.textContent = '导出完整 ZIP';
    button.onclick = function () { yyExportRuntimePlugin(plugin.id); };
    row.append(name, button);
    card.appendChild(row);
  });
  host.appendChild(card);
};

async function yyExportRuntimePlugin(id) {
  var path = '/api/youyuan/runtime/export?id=' + encodeURIComponent(id);
  if (window.android && android.exportFile) {
    var result = await A('POST', path, {});
    if (!result.ok) { T(result.error || '导出失败', 'e'); return; }
    android.exportFile(result.src_path, id + '.zip');
    return;
  }
  var response = await fetch(path);
  if (!response.ok || !response.headers.get('Content-Type')?.includes('application/zip')) {
    T('导出失败：' + await response.text(), 'e');
    return;
  }
  var url = URL.createObjectURL(await response.blob());
  var link = document.createElement('a');
  link.href = url;
  link.download = id + '.zip';
  link.click();
  setTimeout(function () { URL.revokeObjectURL(url); }, 60000);
}
