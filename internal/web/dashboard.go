package web

// dashboardHTML 内置仪表盘：深色、无构建、手机友好。含"总览 / 媒体库 / 设置"三个 Tab。
const dashboardHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>StrmSub · 字幕管家</title>
<style>
:root{--bg:#0f1115;--card:#171a21;--line:#262b36;--txt:#e8eaf0;--dim:#9aa3b2;--acc:#4f8cff;--ok:#3ecf6f;--warn:#f5a623;--bad:#f4585a}
*{box-sizing:border-box;margin:0;padding:0}
body{background:var(--bg);color:var(--txt);font-family:-apple-system,BlinkMacSystemFont,"PingFang SC","Microsoft YaHei",sans-serif;padding:20px;max-width:1080px;margin:0 auto}
header{display:flex;align-items:center;justify-content:space-between;margin-bottom:16px;flex-wrap:wrap;gap:12px}
h1{font-size:22px;font-weight:700}
h1 span{color:var(--acc)}
h2{font-size:16px;margin:22px 0 12px;color:var(--txt)}
.tabs{display:flex;gap:8px;margin-bottom:20px}
.tab{background:var(--card);border:1px solid var(--line);color:var(--dim);border-radius:10px;padding:9px 26px;font-size:15px;cursor:pointer}
.tab.on{background:var(--acc);border-color:var(--acc);color:#fff}
button{background:var(--acc);color:#fff;border:0;border-radius:10px;padding:10px 22px;font-size:15px;cursor:pointer}
button:disabled{opacity:.5}
button.ghost{background:var(--card);border:1px solid var(--line);color:var(--txt)}
button.small{padding:6px 14px;font-size:13px;border-radius:8px}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:12px;margin-bottom:20px}
.card{background:var(--card);border:1px solid var(--line);border-radius:14px;padding:16px}
.card .n{font-size:26px;font-weight:700}
.card .l{color:var(--dim);font-size:13px;margin-top:4px}
.envline{color:var(--dim);font-size:13px;margin-bottom:20px;line-height:1.8}
.envline b{color:var(--txt)}
table{width:100%;border-collapse:collapse;background:var(--card);border-radius:14px;overflow:hidden;font-size:14px}
th,td{padding:12px 14px;text-align:left;border-bottom:1px solid var(--line)}
th{color:var(--dim);font-weight:500;font-size:13px}
.badge{display:inline-block;padding:3px 10px;border-radius:20px;font-size:12px;white-space:nowrap}
.ok{background:rgba(62,207,111,.15);color:var(--ok)}
.missing{background:rgba(245,166,35,.15);color:var(--warn)}
.failed{background:rgba(244,88,90,.15);color:var(--bad)}
.na{background:rgba(154,163,178,.15);color:var(--dim)}
.info{background:rgba(79,140,255,.15);color:var(--acc)}
.src{color:var(--dim);font-size:12px}
.path{color:var(--dim);font-size:12px;max-width:280px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.empty{text-align:center;color:var(--dim);padding:40px}
.form{background:var(--card);border:1px solid var(--line);border-radius:14px;padding:22px;max-width:640px;margin-bottom:20px}
.row{display:flex;align-items:center;justify-content:space-between;gap:14px;padding:13px 0;border-bottom:1px solid var(--line)}
.row:last-child{border-bottom:0}
.row label{font-size:15px;flex-shrink:0}
.row .hint{font-size:12px;color:var(--dim);margin-top:4px}
.row input[type=text],.row input[type=password],.row input[type=number]{background:var(--bg);border:1px solid var(--line);color:var(--txt);border-radius:10px;padding:10px 14px;font-size:15px;width:280px;max-width:55vw}
.row select{background:var(--bg);border:1px solid var(--line);color:var(--txt);border-radius:10px;padding:10px 14px;font-size:15px}
.switch{position:relative;width:52px;height:30px;flex-shrink:0;cursor:pointer}
.switch input{opacity:0;width:0;height:0}
.sl{position:absolute;inset:0;background:#3a4150;border-radius:20px;transition:.2s}
.sl:before{position:absolute;content:"";height:24px;width:24px;left:3px;top:3px;background:#fff;border-radius:50%;transition:.2s}
.switch input:checked + .sl{background:var(--ok)}
.switch input:checked + .sl:before{transform:translateX(22px)}
.btnrow{display:flex;gap:12px;margin-top:18px;flex-wrap:wrap}
#testOut{background:var(--bg);border:1px solid var(--line);border-radius:10px;padding:14px;margin-top:14px;font-size:13px;white-space:pre-wrap;word-break:break-all;max-height:320px;overflow:auto;display:none}
#testOut.show{display:block}
#testOut.err{border-color:var(--bad);color:var(--bad)}
#testOut.good{border-color:var(--ok)}
.toolbar{display:flex;gap:10px;margin-bottom:14px;flex-wrap:wrap;align-items:center}
.toolbar input[type=text]{background:var(--card);border:1px solid var(--line);color:var(--txt);border-radius:10px;padding:10px 14px;font-size:15px;flex:1;min-width:180px}
@media(max-width:640px){.path{display:none}}
</style>
</head>
<body>
<header>
  <h1>🎬 StrmSub <span>字幕管家</span></h1>
  <button id="scanBtn" onclick="triggerScan()">立即扫描</button>
</header>
<div class="tabs">
  <button class="tab on" id="tabOv" onclick="showTab('ov')">总览</button>
  <button class="tab" id="tabLib" onclick="showTab('lib')">媒体库</button>
  <button class="tab" id="tabSet" onclick="showTab('set')">⚙️ 设置</button>
</div>

<div id="viewOv">
  <div class="cards" id="cards"></div>
  <div class="envline" id="envline"></div>
  <table>
    <thead><tr><th>影片</th><th>年份</th><th>字幕状态</th><th>来源</th><th class="path">文件</th></tr></thead>
    <tbody id="rows"><tr><td colspan="5" class="empty">加载中…</td></tr></tbody>
  </table>
</div>

<div id="viewLib" style="display:none">
  <div class="toolbar">
    <input type="text" id="libFilter" placeholder="搜索片名…" oninput="renderLib()">
    <button class="ghost small" onclick="loadLib()">刷新</button>
  </div>
  <table>
    <thead><tr><th>影片</th><th>类型</th><th>年份</th><th>IMDb</th><th>字幕状态</th><th>操作</th></tr></thead>
    <tbody id="libRows"><tr><td colspan="6" class="empty">加载中…</td></tr></tbody>
  </table>
</div>

<div id="viewSet" style="display:none">
  <h2>飞牛影视 API</h2>
  <div class="form">
    <div class="row">
      <div><label>是否开启飞牛影视 API</label><div class="hint">开启后，用飞牛刮削好的数据识别媒体、找字幕（Emby 式接入）</div></div>
      <label class="switch"><input type="checkbox" id="fEnabled"><span class="sl"></span></label>
    </div>
    <div class="row">
      <div><label>飞牛服务器 URL</label><div class="hint">如 http://192.168.1.10:8005</div></div>
      <input type="text" id="fUrl" placeholder="http://<飞牛IP>:8005">
    </div>
    <div class="row">
      <div><label>用户名</label></div>
      <input type="text" id="fUser" placeholder="飞牛用户名">
    </div>
    <div class="row">
      <div><label>密码</label><div class="hint">保存在本机，不会回传</div></div>
      <input type="password" id="fPass" placeholder="留空=不修改">
    </div>
    <div class="row">
      <div><label>路径映射</label><div class="hint">飞牛侧路径 → 本机路径，逗号分隔<br>如 /vol1/media:/media</div></div>
      <input type="text" id="fPathMap" placeholder="/vol1/media:/media">
    </div>
    <div class="btnrow">
      <button class="ghost" id="testBtn" onclick="testFnos()">检测服务</button>
    </div>
    <div id="testOut"></div>
  </div>

  <h2>字幕源与扫描</h2>
  <div class="form">
    <div class="row">
      <div><label>ASSRT Token</label><div class="hint">射手网 https://assrt.net/usercp.php 免费获取，20次/分钟</div></div>
      <input type="password" id="sAssrt" placeholder="未设置">
    </div>
    <div class="row">
      <div><label>OpenSubtitles API Key</label><div class="hint">https://www.opensubtitles.com 注册后获取</div></div>
      <input type="text" id="sOsKey" placeholder="未设置">
    </div>
    <div class="row">
      <div><label>OpenSubtitles 用户名</label></div>
      <input type="text" id="sOsUser" placeholder="未设置">
    </div>
    <div class="row">
      <div><label>OpenSubtitles 密码</label><div class="hint">保存在本机，不会回传</div></div>
      <input type="password" id="sOsPass" placeholder="留空=不修改">
    </div>
    <div class="row">
      <div><label>SubDL Key</label><div class="hint">预留，暂未启用</div></div>
      <input type="text" id="sSubdl" placeholder="未设置">
    </div>
    <div class="row">
      <div><label>扫描间隔（分钟）</label><div class="hint">定时全量扫描，保存后立即生效</div></div>
      <input type="number" id="sInterval" min="5" max="1440" placeholder="30">
    </div>
    <div class="row">
      <div><label>目标语言</label><div class="hint">保存后立即生效</div></div>
      <select id="sLang">
        <option value="zh-Hans">简体中文</option>
        <option value="zh-Hant">繁体中文</option>
      </select>
    </div>
    <div class="btnrow">
      <button id="saveBtn" onclick="saveSettings()">保存全部设置</button>
    </div>
  </div>
</div>

<script>
var libData = [];
function showTab(t){
  document.getElementById('viewOv').style.display = t==='ov'?'':'none';
  document.getElementById('viewLib').style.display = t==='lib'?'':'none';
  document.getElementById('viewSet').style.display = t==='set'?'':'none';
  document.getElementById('tabOv').classList.toggle('on', t==='ov');
  document.getElementById('tabLib').classList.toggle('on', t==='lib');
  document.getElementById('tabSet').classList.toggle('on', t==='set');
  if(t==='set') loadSettings();
  if(t==='lib') loadLib();
}
function mediaName(m){
  if(m.Type==='episode') return m.Title+' S'+String(m.Season).padStart(2,'0')+'E'+String(m.Episode).padStart(2,'0');
  return m.Title;
}
function statusBadge(st){
  if(st==='ok') return '<span class="badge ok">✓ 已有</span>';
  if(st==='missing') return '<span class="badge missing">无合适字幕</span>';
  if(st==='failed') return '<span class="badge failed">失败</span>';
  return '<span class="badge na">未扫描</span>';
}
async function load(){
  const [st, media] = await Promise.all([
    fetch('/api/status').then(r=>r.json()),
    fetch('/api/media').then(r=>r.json())
  ]);
  document.getElementById('cards').innerHTML =
    card(st.total,'媒体总数')+card(st.downloaded,'已下载','var(--ok)')+
    card(st.skipped,'已有字幕')+card(st.missing,'无合适字幕','var(--warn)')+card(st.failed,'失败','var(--bad)');
  var prov = (st.providers||[]).join('、')||'无';
  var srcs = (st.sources||[]).map(function(s){ return s.name+(s.enabled?' ✓':' ✗'); }).join(' · ')||'无';
  document.getElementById('envline').innerHTML =
    '元数据源：<b>'+esc(prov)+'</b>　字幕源：<b>'+esc(srcs)+'</b><br>目标语言：<b>'+esc(st.targetLang||'-')+'</b>　扫描间隔：<b>'+esc(st.scanInterval||'-')+'</b>';
  const tb = document.getElementById('rows');
  if(!media || !media.length){ tb.innerHTML='<tr><td colspan="5" class="empty">暂无媒体，请点"立即扫描"</td></tr>'; return; }
  tb.innerHTML = media.slice(0,200).map(m=>
    '<tr><td>'+esc(mediaName(m))+'</td><td>'+(m.Year||'-')+'</td><td>'+statusBadge(m.SubStatus)+'</td>'+
    '<td class="src">'+esc(m.SubSource||'-')+'</td><td class="path">'+esc(m.FilePath||'')+'</td></tr>'
  ).join('');
}
function card(n,l,c){ return '<div class="card"><div class="n" style="color:'+(c||'var(--txt)')+'">'+(n||0)+
  '</div><div class="l">'+l+'</div></div>'; }
function esc(s){ return String(s==null?'':s).replace(/[&<>"]/g, c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c])); }
async function triggerScan(){
  const b=document.getElementById('scanBtn'); b.disabled=true; b.textContent='扫描中…';
  await fetch('/api/scan',{method:'POST'});
  setTimeout(()=>{ b.disabled=false; b.textContent='立即扫描'; load(); }, 8000);
}
async function loadLib(){
  const tb = document.getElementById('libRows');
  tb.innerHTML = '<tr><td colspan="6" class="empty">加载中…</td></tr>';
  try{
    libData = await fetch('/api/media').then(r=>r.json());
  }catch(e){ libData = []; }
  renderLib();
}
function renderLib(){
  const tb = document.getElementById('libRows');
  const q = document.getElementById('libFilter').value.trim().toLowerCase();
  var list = libData || [];
  if(q) list = list.filter(m=>(m.Title||'').toLowerCase().indexOf(q)>=0);
  if(!list.length){ tb.innerHTML='<tr><td colspan="6" class="empty">暂无媒体，请先点"立即扫描"</td></tr>'; return; }
  tb.innerHTML = list.slice(0,500).map(m=>{
    const typeBadge = m.Type==='episode' ? '<span class="badge info">剧集</span>' : '<span class="badge info">电影</span>';
    return '<tr><td>'+esc(mediaName(m))+'</td><td>'+typeBadge+'</td><td>'+(m.Year||'-')+'</td>'+
      '<td class="src">'+esc(m.ImdbID||'-')+'</td><td>'+statusBadge(m.SubStatus)+'</td>'+
      '<td><button class="small" data-id="'+esc(m.ID)+'" onclick="searchOne(this)">搜字幕</button></td></tr>';
  }).join('');
}
async function searchOne(btn){
  const id = btn.getAttribute('data-id');
  btn.disabled = true; const old = btn.textContent; btn.textContent = '搜索中…';
  try{
    const r = await fetch('/api/search-one',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({id:id})});
    const j = await r.json();
    alert(j.ok ? j.message : ('失败：'+(j.detail||'未知错误')));
    loadLib(); load();
  }catch(e){ alert('请求失败：'+e.message); }
  btn.disabled = false; btn.textContent = old;
}
async function loadSettings(){
  const s = await fetch('/api/settings').then(r=>r.json());
  document.getElementById('fEnabled').checked = !!s.enabled;
  document.getElementById('fUrl').value = s.url||'';
  document.getElementById('fUser').value = s.user||'';
  document.getElementById('fPathMap').value = s.pathMap||'';
  document.getElementById('fPass').value = '';
  document.getElementById('fPass').placeholder = s.hasPass ? '已保存，留空=不修改' : '未设置';
  document.getElementById('sAssrt').value = s.assrtToken||'';
  document.getElementById('sOsKey').value = s.osApiKey||'';
  document.getElementById('sOsUser').value = s.osUser||'';
  document.getElementById('sSubdl').value = s.subdlKey||'';
  document.getElementById('sOsPass').value = '';
  document.getElementById('sOsPass').placeholder = s.hasOsPass ? '已保存，留空=不修改' : '未设置';
  document.getElementById('sInterval').value = s.scanIntervalMin||'';
  document.getElementById('sInterval').placeholder = '30';
  document.getElementById('sLang').value = s.targetLang||'zh-Hans';
}
function settingsBody(){
  return {
    enabled: document.getElementById('fEnabled').checked,
    url: document.getElementById('fUrl').value,
    user: document.getElementById('fUser').value,
    pass: document.getElementById('fPass').value,
    pathMap: document.getElementById('fPathMap').value,
    assrtToken: document.getElementById('sAssrt').value,
    osApiKey: document.getElementById('sOsKey').value,
    osUser: document.getElementById('sOsUser').value,
    osPass: document.getElementById('sOsPass').value,
    subdlKey: document.getElementById('sSubdl').value,
    scanIntervalMinutes: parseInt(document.getElementById('sInterval').value,10)||0,
    targetLang: document.getElementById('sLang').value
  };
}
async function saveSettings(){
  const b=document.getElementById('saveBtn'); b.disabled=true; b.textContent='保存中…';
  try{
    const r = await fetch('/api/settings',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(settingsBody())});
    if(!r.ok) throw new Error(await r.text());
    b.textContent='已保存 ✓';
    loadSettings(); load();
  }catch(e){ b.textContent='保存失败'; alert('保存失败: '+e.message); }
  setTimeout(()=>{ b.disabled=false; b.textContent='保存全部设置'; }, 1500);
}
async function testFnos(){
  const b=document.getElementById('testBtn'); b.disabled=true; b.textContent='检测中…';
  const out=document.getElementById('testOut'); out.className=''; out.textContent='正在连接飞牛服务器…';
  const body = settingsBody();
  try{
    const r = await fetch('/api/fnos/test',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
    const j = await r.json();
    out.className = 'show '+(j.ok?'good':'err');
    out.textContent = (j.ok?'✅ 检测成功':'')+(j.saved||'')+'\n'+(j.detail||JSON.stringify(j));
    if(j.ok){ loadSettings(); load(); }
  }catch(e){ out.className='show err'; out.textContent='请求失败: '+e.message; }
  b.disabled=false; b.textContent='检测服务';
}
load(); setInterval(load, 15000);
</script>
</body>
</html>`
