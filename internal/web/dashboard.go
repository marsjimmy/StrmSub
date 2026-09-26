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
/* ---- 媒体库：海报墙 ---- */
.subtabs{display:flex;gap:8px;margin-bottom:12px}
.subtab{background:var(--card);border:1px solid var(--line);color:var(--dim);border-radius:20px;padding:8px 20px;font-size:14px;cursor:pointer}
.subtab.on{background:var(--acc);border-color:var(--acc);color:#fff}
.crumb{font-size:13px;color:var(--dim);margin-bottom:12px}
.crumb a{color:var(--acc);cursor:pointer;text-decoration:none}
.crumb a:hover{text-decoration:underline}
.grid5{display:grid;grid-template-columns:repeat(5,1fr);gap:14px}
@media(max-width:900px){.grid5{grid-template-columns:repeat(3,1fr)}}
@media(max-width:600px){.grid5{grid-template-columns:repeat(2,1fr);gap:10px}}
.pcard{background:var(--card);border:1px solid var(--line);border-radius:12px;overflow:hidden;cursor:pointer;transition:transform .15s}
.pcard:hover{transform:translateY(-3px);border-color:var(--acc)}
.pimg{position:relative;aspect-ratio:2/3;background:#232838;display:flex;align-items:center;justify-content:center;overflow:hidden}
.pimg .pt{font-size:15px;font-weight:700;color:var(--dim);padding:0 12px;text-align:center;line-height:1.5;display:-webkit-box;-webkit-line-clamp:4;-webkit-box-orient:vertical;overflow:hidden}
.pimg img{position:absolute;inset:0;width:100%;height:100%;object-fit:cover}
.pmeta{padding:10px 12px}
.pmeta .t{font-size:14px;font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.pmeta .s{font-size:12px;color:var(--dim);margin-top:4px}
.shead{display:flex;gap:16px;margin-bottom:16px;background:var(--card);border:1px solid var(--line);border-radius:12px;padding:14px}
.shead .pimg{width:110px;flex-shrink:0;border-radius:8px}
.shead .info{flex:1;min-width:0}
.shead .info h3{margin:0 0 6px;font-size:17px}
.shead .info .ov{font-size:13px;color:var(--dim);line-height:1.7;display:-webkit-box;-webkit-line-clamp:5;-webkit-box-orient:vertical;overflow:hidden}
.eprow{display:flex;align-items:center;gap:12px;background:var(--card);border:1px solid var(--line);border-radius:10px;padding:12px 14px;margin-bottom:10px}
.eprow .ep{flex-shrink:0;background:#232838;border-radius:8px;padding:6px 10px;font-size:13px;font-weight:700}
.eprow .ov{flex:1;font-size:13px;color:var(--dim);line-height:1.6;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;min-width:0}
.modal{position:fixed;inset:0;background:rgba(0,0,0,.65);display:flex;align-items:center;justify-content:center;z-index:50;padding:20px}
.mbox{background:var(--card);border:1px solid var(--line);border-radius:16px;max-width:560px;width:100%;max-height:85vh;overflow:auto;padding:20px}
.mbox .shead{background:none;border:0;padding:0}
.mrow{display:flex;gap:10px;margin-top:14px;align-items:center;flex-wrap:wrap}
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
  <div class="subtabs">
    <button class="subtab on" id="subMovies" onclick="setLibTab('movies')">🎬 电影</button>
    <button class="subtab" id="subSeries" onclick="setLibTab('series')">📺 电视剧</button>
  </div>
  <div class="toolbar">
    <input type="text" id="libFilter" placeholder="搜索片名…" oninput="renderLibView()">
    <button class="ghost small" onclick="renderLibView(true)">刷新</button>
  </div>
  <div class="crumb" id="libCrumb"></div>
  <div id="libBox"><div class="empty">加载中…</div></div>
</div>
<div class="modal" id="movieModal" style="display:none" onclick="if(event.target===this)closeMovie()">
  <div class="mbox" id="movieBox"></div>
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
      <div><label>SubHD / 迅雷字幕</label><div class="hint">免 key，已自动启用：SubHD 按标题搜索（走站内预览接口下载），迅雷按视频特征 CID 查询（.strm 指向 http(s) 且支持 Range 时有效）</div></div>
      <div style="color:#7d8590;font-size:13px">已启用 ✓</div>
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
var libTab = 'movies';       // movies | series
var libSeriesId = null, libSeriesTitle = '', libSeriesPoster = '', libSeriesOv = '', libSeason = 0;
var libMovies = [], libSeries = [];
function showTab(t){
  document.getElementById('viewLib').style.display = t==='lib'?'':'none';
  document.getElementById('viewOv').style.display = t==='ov'?'':'none';
  document.getElementById('viewSet').style.display = t==='set'?'':'none';
  document.getElementById('tabOv').classList.toggle('on', t==='ov');
  document.getElementById('tabLib').classList.toggle('on', t==='lib');
  document.getElementById('tabSet').classList.toggle('on', t==='set');
  if(t==='set') loadSettings();
  if(t==='lib') renderLibView(true);
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
  var show = (media||[]).filter(m=>m.Type==='movie'||m.Type==='episode');
  if(!show.length){ tb.innerHTML='<tr><td colspan="5" class="empty">暂无媒体，请点"立即扫描"</td></tr>'; return; }
  tb.innerHTML = show.slice(0,200).map(m=>
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
function setLibTab(t){
  libTab = t; libSeriesId = null; libSeason = 0;
  document.getElementById('subMovies').classList.toggle('on', t==='movies');
  document.getElementById('subSeries').classList.toggle('on', t==='series');
  renderLibView(true);
}
function libCrumbHTML(){
  var h = '<a onclick="libBack(\'root\')">媒体库</a>';
  if(libTab==='movies') h += ' / 电影';
  else h += ' / <a onclick="libBack(\'series\')">电视剧</a>';
  if(libSeriesId) h += ' / <a onclick="libBack(\'seasons\')">'+esc(libSeriesTitle)+'</a>';
  if(libSeason) h += ' / 第 '+libSeason+' 季';
  return h;
}
function libBack(where){
  if(where==='root'){ libSeriesId=null; libSeason=0; }
  if(where==='series'){ libTab='series'; libSeriesId=null; libSeason=0;
    document.getElementById('subMovies').classList.toggle('on', false);
    document.getElementById('subSeries').classList.toggle('on', true); }
  if(where==='seasons'){ libSeason=0; }
  renderLibView();
}
function posterImg(poster, title){
  if(poster) return '<img loading="lazy" src="'+poster+'" alt="" onerror="this.remove()">';
  return '';
}
function pcardHTML(it, onclick){
  return '<div class="pcard" onclick="'+onclick+'"><div class="pimg"><span class="pt">'+esc(it.title)+'</span>'+
    posterImg(it.poster, it.title)+'</div><div class="pmeta"><div class="t">'+esc(it.title)+'</div>'+
    '<div class="s">'+(it.year||'')+'</div></div></div>';
}
async function renderLibView(force){
  const box = document.getElementById('libBox');
  document.getElementById('libCrumb').innerHTML = libCrumbHTML();
  const q = document.getElementById('libFilter').value.trim().toLowerCase();
  const match = it => !q || (it.title||'').toLowerCase().indexOf(q)>=0;
  box.innerHTML = '<div class="empty">加载中…</div>';
  try{
    if(libTab==='movies' && !libSeriesId){
      if(force || !libMovies.length) libMovies = await fetch('/api/library/movies').then(r=>r.json());
      var list = (libMovies||[]).filter(match);
      if(!list.length){ box.innerHTML='<div class="empty">暂无电影，请先点"立即扫描"</div>'; return; }
      box.innerHTML = '<div class="grid5">'+list.map(m=>pcardHTML(m, "openMovie('"+m.id+"')")).join('')+'</div>';
    }else if(libTab==='series' && !libSeriesId){
      if(force || !libSeries.length) libSeries = await fetch('/api/library/series').then(r=>r.json());
      var sl = (libSeries||[]).filter(match);
      if(!sl.length){ box.innerHTML='<div class="empty">暂无电视剧，请先点"立即扫描"</div>'; return; }
      box.innerHTML = '<div class="grid5">'+sl.map(s=>{
        var sub = (s.seasons?s.seasons+' 季':'')+(s.seasons&&s.episodes?' · ':'')+(s.episodes?s.episodes+' 集':'');
        return '<div class="pcard" onclick="openSeries(\''+s.id+'\')"><div class="pimg"><span class="pt">'+esc(s.title)+'</span>'+
          posterImg(s.poster, s.title)+'</div><div class="pmeta"><div class="t">'+esc(s.title)+'</div>'+
          '<div class="s">'+(s.year||'')+(sub?' · '+sub:'')+'</div></div></div>';
      }).join('')+'</div>';
    }else if(libSeriesId && !libSeason){
      var seasons = await fetch('/api/library/seasons?series='+encodeURIComponent(libSeriesId)).then(r=>r.json());
      var head = '<div class="shead"><div class="pimg"><span class="pt">'+esc(libSeriesTitle)+'</span>'+
        posterImg(libSeriesPoster, libSeriesTitle)+'</div><div class="info"><h3>'+esc(libSeriesTitle)+'</h3>'+
        '<div class="ov">'+esc(libSeriesOv||'暂无简介')+'</div></div></div>';
      if(!seasons.length){ box.innerHTML = head+'<div class="empty">暂无季信息</div>'; return; }
      box.innerHTML = head+'<div class="grid5">'+seasons.map(sn=>pcardHTML(
        {title:'第 '+sn.season+' 季', year:'', poster:sn.poster}, "openSeason("+sn.season+")"
      )).join('')+'</div>';
    }else{
      var eps = await fetch('/api/library/episodes?series='+encodeURIComponent(libSeriesId)+'&season='+libSeason).then(r=>r.json());
      if(!eps.length){ box.innerHTML='<div class="empty">这一季暂无剧集</div>'; return; }
      box.innerHTML = eps.map(e=>{
        var code = 'S'+String(e.season).padStart(2,'0')+'E'+String(e.episode).padStart(2,'0');
        return '<div class="eprow"><div class="ep">'+code+'</div><div class="ov">'+esc(e.overview||'暂无简介')+'</div>'+
          statusBadge(e.subStatus)+'<button class="small" data-id="'+esc(e.id)+'" onclick="searchOne(this)">搜字幕</button></div>';
      }).join('');
    }
  }catch(e){ box.innerHTML='<div class="empty">加载失败：'+esc(e.message)+'</div>'; }
}
function openMovie(id){
  var m = (libMovies||[]).find(x=>x.id===id);
  if(!m) return;
  document.getElementById('movieBox').innerHTML =
    '<div class="shead"><div class="pimg"><span class="pt">'+esc(m.title)+'</span>'+posterImg(m.poster,m.title)+'</div>'+
    '<div class="info"><h3>'+esc(m.title)+(m.year?' ('+m.year+')':'')+'</h3>'+
    '<div class="ov">'+esc(m.overview||'暂无简介')+'</div></div></div>'+
    '<div class="mrow">'+statusBadge(m.subStatus)+
    '<button class="small" data-id="'+esc(m.id)+'" onclick="searchOne(this)">搜字幕</button>'+
    '<button class="ghost small" onclick="closeMovie()">关闭</button></div>';
  document.getElementById('movieModal').style.display = 'flex';
}
function closeMovie(){ document.getElementById('movieModal').style.display = 'none'; }
function openSeries(id){
  var s = (libSeries||[]).find(x=>x.id===id);
  if(!s) return;
  libSeriesId = id; libSeriesTitle = s.title; libSeriesPoster = s.poster; libSeriesOv = s.overview; libSeason = 0;
  renderLibView(true);
}
function openSeason(n){ libSeason = n; renderLibView(true); }
async function searchOne(btn){
  const id = btn.getAttribute('data-id');
  btn.disabled = true; const old = btn.textContent; btn.textContent = '搜索中…';
  try{
    const r = await fetch('/api/search-one',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({id:id})});
    const j = await r.json();
    alert(j.ok ? j.message : ('失败：'+(j.detail||'未知错误')));
    closeMovie(); renderLibView(true); load();
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
