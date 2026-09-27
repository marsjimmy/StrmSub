package web

// dashboardHTML 内置 Web 界面：深色、响应式、侧边栏可收起。
// __API_TOKEN__ 由服务端替换为自动生成的 API 令牌。
var dashboardHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>StrmSub</title>
<style>
:root{
  --bg:#0f1115; --bg2:#161a21; --bg3:#1e242e; --line:#262d38;
  --text:#e6e9ef; --muted:#9aa3b2; --accent:#3b82f6; --accent2:#2563eb;
  --green:#22c55e; --red:#ef4444; --orange:#f59e0b; --radius:10px;
}
*{box-sizing:border-box;margin:0;padding:0}
body{background:var(--bg);color:var(--text);font-family:-apple-system,"PingFang SC","Microsoft YaHei",system-ui,sans-serif;font-size:14px;min-height:100vh}
#app{display:flex;min-height:100vh}
#sidebar{width:210px;background:var(--bg2);border-right:1px solid var(--line);padding:16px 10px;display:flex;flex-direction:column;gap:4px;position:sticky;top:0;height:100vh;transition:margin .2s}
#sidebar.hidden{margin-left:-210px}
.logo{font-size:18px;font-weight:700;padding:6px 12px 16px;color:var(--text)}
.logo span{color:var(--accent)}
.nav{display:flex;flex-direction:column;gap:2px}
.nav a{color:var(--muted);text-decoration:none;padding:10px 12px;border-radius:8px;display:flex;gap:10px;align-items:center}
.nav a:hover{background:var(--bg3);color:var(--text)}
.nav a.active{background:var(--accent2);color:#fff}
.nav .foot{margin-top:auto;font-size:12px;color:var(--muted);padding:10px 12px;line-height:1.6}
#main{flex:1;min-width:0;display:flex;flex-direction:column}
#topbar{display:flex;align-items:center;gap:10px;padding:12px 18px;border-bottom:1px solid var(--line);background:var(--bg2);position:sticky;top:0;z-index:5}
#burger{background:var(--bg3);border:1px solid var(--line);color:var(--text);border-radius:8px;padding:6px 10px;cursor:pointer;font-size:16px}
#pageTitle{font-size:16px;font-weight:600;flex:1}
#content{padding:18px;max-width:1200px;width:100%;margin:0 auto}
.page{display:none}
.page.on{display:block}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:12px;margin-bottom:18px}
.card{background:var(--bg2);border:1px solid var(--line);border-radius:var(--radius);padding:16px}
.card .num{font-size:26px;font-weight:700}
.card .label{color:var(--muted);font-size:12px;margin-top:4px}
.btn{background:var(--accent);color:#fff;border:none;border-radius:8px;padding:8px 16px;cursor:pointer;font-size:14px}
.btn:hover{background:var(--accent2)}
.btn.ghost{background:var(--bg3);border:1px solid var(--line)}
.btn.small{padding:5px 10px;font-size:12px}
.btn:disabled{opacity:.5;cursor:default}
input[type=text],input[type=password],input[type=number],select{background:var(--bg3);border:1px solid var(--line);color:var(--text);border-radius:8px;padding:8px 10px;font-size:14px;width:100%}
textarea{background:var(--bg3);border:1px solid var(--line);color:var(--text);border-radius:8px;padding:8px 10px;font-size:13px;width:100%;font-family:monospace}
.toolbar{display:flex;gap:8px;margin-bottom:14px;flex-wrap:wrap}
.toolbar input[type=text]{max-width:320px}
.mgrid{display:grid;grid-template-columns:repeat(auto-fill,minmax(130px,1fr));gap:14px}
.mcard{background:var(--bg2);border:1px solid var(--line);border-radius:var(--radius);overflow:hidden;cursor:pointer;transition:transform .12s}
.mcard:hover{transform:translateY(-2px);border-color:var(--accent)}
.mcard .cover{aspect-ratio:2/3;background:var(--bg3);display:flex;align-items:center;justify-content:center;overflow:hidden}
.mcard .cover img{width:100%;height:100%;object-fit:cover}
.mcard .cover .noimg{color:var(--muted);font-size:12px;text-align:center;padding:8px}
.mcard .info{padding:8px 10px}
.mcard .t{font-size:13px;font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.mcard .m{font-size:11px;color:var(--muted);margin-top:3px}
.badge{display:inline-block;font-size:11px;border-radius:20px;padding:2px 8px;margin-top:5px}
.badge.ok{background:rgba(34,197,94,.15);color:var(--green)}
.badge.missing{background:rgba(239,68,68,.15);color:var(--red)}
.badge.failed{background:rgba(245,158,11,.15);color:var(--orange)}
.badge.unknown{background:rgba(154,163,178,.15);color:var(--muted)}
.srcgroup{background:var(--bg2);border:1px solid var(--line);border-radius:var(--radius);margin-bottom:12px;overflow:hidden}
.srcgroup .hd{background:var(--bg3);padding:10px 14px;font-weight:600;display:flex;justify-content:space-between;align-items:center}
.hit{display:flex;align-items:center;gap:10px;padding:10px 14px;border-top:1px solid var(--line);flex-wrap:wrap}
.hit .nm{flex:1;min-width:180px}
.hit .nm .fn{font-size:13px;word-break:break-all}
.hit .nm .dt{font-size:11px;color:var(--muted);margin-top:2px}
.tag{font-size:11px;background:var(--bg3);border:1px solid var(--line);border-radius:6px;padding:2px 6px;color:var(--muted)}
.tag.cached{color:var(--green);border-color:var(--green)}
table{width:100%;border-collapse:collapse;background:var(--bg2);border-radius:var(--radius);overflow:hidden}
th,td{padding:10px 12px;text-align:left;border-bottom:1px solid var(--line);font-size:13px}
th{color:var(--muted);font-weight:500;font-size:12px}
tr:last-child td{border-bottom:none}
.sec{background:var(--bg2);border:1px solid var(--line);border-radius:var(--radius);padding:16px;margin-bottom:16px}
.sec h3{font-size:15px;margin-bottom:12px}
.row{display:flex;gap:10px;align-items:center;margin-bottom:10px;flex-wrap:wrap}
.row label{min-width:110px;color:var(--muted);font-size:13px}
.row .grow{flex:1;min-width:200px}
.switch{position:relative;width:40px;height:22px;flex:none}
.switch input{opacity:0;width:0;height:0}
.switch .sl{position:absolute;inset:0;background:var(--bg3);border:1px solid var(--line);border-radius:20px;cursor:pointer;transition:.15s}
.switch .sl:before{content:"";position:absolute;width:16px;height:16px;left:2px;top:2px;background:var(--muted);border-radius:50%;transition:.15s}
.switch input:checked + .sl{background:var(--accent2)}
.switch input:checked + .sl:before{transform:translateX(18px);background:#fff}
.rule{border:1px solid var(--line);border-radius:8px;padding:10px 12px;margin-bottom:8px;background:var(--bg3)}
.rule .rp{font-family:monospace;font-size:12px;color:var(--accent);word-break:break-all}
.hint{font-size:12px;color:var(--muted);line-height:1.7;margin-top:8px}
#modalWrap{display:none;position:fixed;inset:0;background:rgba(0,0,0,.6);z-index:50;align-items:center;justify-content:center;padding:16px}
#modalWrap.on{display:flex}
#modal{background:var(--bg2);border:1px solid var(--line);border-radius:12px;max-width:760px;width:100%;max-height:90vh;overflow:auto;padding:18px}
#modal .x{float:right;background:var(--bg3);border:1px solid var(--line);color:var(--text);border-radius:8px;padding:4px 10px;cursor:pointer}
.spin{display:inline-block;width:14px;height:14px;border:2px solid var(--line);border-top-color:var(--accent);border-radius:50%;animation:sp 0.8s linear infinite;vertical-align:-2px}
@keyframes sp{to{transform:rotate(360deg)}}
.note{font-size:12px;color:var(--muted);margin-top:6px;line-height:1.6}
code{background:var(--bg3);border:1px solid var(--line);border-radius:4px;padding:1px 5px;font-size:12px}
.empty{color:var(--muted);text-align:center;padding:30px}
@media(max-width:700px){
  #sidebar{position:fixed;z-index:20;transform:translateX(-100%);transition:transform .2s}
  #sidebar.hidden{margin-left:0;transform:translateX(-100%)}
  #sidebar.open{transform:translateX(0)}
  #content{padding:12px}
  .mgrid{grid-template-columns:repeat(auto-fill,minmax(100px,1fr))}
}
</style>
</head>
<body>
<div id="app">
  <aside id="sidebar">
    <div class="logo">Strm<span>Sub</span> v2</div>
    <nav class="nav">
      <a href="#/home" data-p="home">🏠 主页</a>
      <a href="#/media" data-p="media">🎬 媒体</a>
      <a href="#/subtitles" data-p="subtitles">📝 字幕</a>
      <a href="#/settings" data-p="settings">⚙️ 设置</a>
    </nav>
    <div class="foot">v2.0.0 · 正则识别<br>· 6 字幕源聚合</div>
  </aside>
  <div id="main">
    <div id="topbar">
      <button id="burger" onclick="toggleSidebar()">☰</button>
      <div id="pageTitle">主页</div>
      <button class="btn small" onclick="doScan(this)">🔄 扫描</button>
    </div>
    <div id="content">
      <div class="page" id="page-home"></div>
      <div class="page" id="page-media"></div>
      <div class="page" id="page-subtitles"></div>
      <div class="page" id="page-settings"></div>
    </div>
  </div>
</div>
<div id="modalWrap"><div id="modal"></div></div>
<script>
var API_TOKEN = "__API_TOKEN__";

/* ---------- 工具 ---------- */
function api(path, opts) {
  opts = opts || {};
  opts.headers = opts.headers || {};
  opts.headers['X-API-Token'] = API_TOKEN;
  if (opts.body && typeof opts.body === 'object') {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(opts.body);
  }
  return fetch(path, opts).then(function(r) {
    if (!r.ok) throw new Error('请求失败 ' + r.status);
    return r.json();
  });
}
function esc(s) {
  return String(s == null ? '' : s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}
function $(id) { return document.getElementById(id); }
function subBadge(st) {
  if (st === 'ok') return '<span class="badge ok">已有字幕</span>';
  if (st === 'failed') return '<span class="badge failed">下载失败</span>';
  if (st === 'missing') return '<span class="badge missing">无合适字幕</span>';
  return '<span class="badge unknown">待找字幕</span>';
}
function mediaMeta(m) {
  var parts = [];
  if (m.year) parts.push(m.year);
  if (m.season > 0 && m.episode > 0) parts.push('S' + m.season + 'E' + m.episode);
  return parts.join(' · ');
}
function coverURL(id) { return '/api/cover?id=' + encodeURIComponent(id) + '&token=' + encodeURIComponent(API_TOKEN); }

/* ---------- 侧边栏 ---------- */
function toggleSidebar() {
  var sb = $('sidebar');
  if (window.innerWidth <= 700) sb.classList.toggle('open');
  else sb.classList.toggle('hidden');
}

/* ---------- 路由 ---------- */
var titles = {home:'主页', media:'媒体', subtitles:'字幕', settings:'设置'};
function route() {
  var p = (location.hash || '#/home').replace('#/','').split('?')[0];
  if (!titles[p]) p = 'home';
  document.querySelectorAll('.page').forEach(function(el){ el.classList.remove('on'); });
  $('page-' + p).classList.add('on');
  document.querySelectorAll('.nav a').forEach(function(a){
    a.classList.toggle('active', a.getAttribute('data-p') === p);
  });
  $('pageTitle').textContent = titles[p];
  if (window.innerWidth <= 700) $('sidebar').classList.remove('open');
  if (p === 'home') renderHome();
  if (p === 'media') renderMedia();
  if (p === 'subtitles') renderSubPage();
  if (p === 'settings') renderSettings();
}
window.addEventListener('hashchange', route);

function doScan(btn) {
  btn.disabled = true;
  api('/api/scan', {method:'POST'}).then(function(){
    btn.textContent = '⏳ 扫描中…';
    setTimeout(function(){ btn.disabled = false; btn.textContent = '🔄 扫描'; route(); }, 4000);
  }).catch(function(e){ alert(e.message); btn.disabled = false; });
}

/* ---------- 主页 ---------- */
function renderHome() {
  var el = $('page-home');
  el.innerHTML = '<div class="cards"><div class="card"><div class="num"><span class="spin"></span></div><div class="label">加载中</div></div></div>';
  api('/api/status').then(function(st){
    var srcs = st.sources.map(function(s){
      return '<span class="tag">' + esc(s.name) + (s.enabled ? ' ✅' : ' ⏸') + '</span>';
    }).join(' ');
    var h = '<div class="cards">';
    h += '<div class="card"><div class="num">' + st.mediaTotal + '</div><div class="label">媒体总数</div></div>';
    h += '<div class="card"><div class="num" style="color:var(--green)">' + st.withSub + '</div><div class="label">已有字幕</div></div>';
    h += '<div class="card"><div class="num" style="color:var(--red)">' + st.missing + '</div><div class="label">待找字幕</div></div>';
    h += '<div class="card"><div class="num" style="font-size:15px">+' + st.lastAdded + ' / ~' + st.lastUpdated + ' / −' + st.lastRemoved + '</div><div class="label">上次扫描 增/改/删</div></div>';
    h += '</div>';
    h += '<div class="sec"><h3>字幕源状态</h3><div>' + srcs + '</div>';
    h += '<div class="hint">媒体目录：' + st.mediaDirs.map(esc).join('；') + '<br>字幕保存：' + esc(st.subtitleDir || '视频同目录') + '<br>目标语言：' + esc(st.targetLang) + ' · 扫描周期：' + esc(st.scanInterval) + '<br>上次扫描：' + esc(st.lastScan || '尚未扫描') + '</div></div>';
    h += '<div class="sec"><h3>最近下载</h3><div id="homeHist"><span class="spin"></span> 加载中…</div></div>';
    el.innerHTML = h;
    api('/api/history?limit=10').then(function(list){
      var t = $('homeHist');
      if (!list || !list.length) { t.innerHTML = '<div class="empty">暂无下载记录</div>'; return; }
      var rows = list.map(function(r){
        var st2 = r.status === 'ok' ? '<span class="badge ok">成功</span>' : '<span class="badge failed">失败</span>';
        return '<tr><td>' + esc(r.media_title || r.filename) + '</td><td>' + esc(r.source) + '</td><td>' + st2 + '</td><td>' + esc(r.created_at) + '</td></tr>';
      }).join('');
      t.innerHTML = '<table><tr><th>标题</th><th>来源</th><th>状态</th><th>时间</th></tr>' + rows + '</table>';
    });
  }).catch(function(e){ el.innerHTML = '<div class="empty">' + esc(e.message) + '</div>'; });
}

/* ---------- 媒体页 ---------- */
var mediaList = [];
function renderMedia() {
  var el = $('page-media');
  el.innerHTML = '<div class="toolbar"><input type="text" id="mq" placeholder="搜索标题…" onkeydown="if(event.key===\'Enter\')loadMedia()"><button class="btn" onclick="loadMedia()">搜索</button><button class="btn ghost" onclick="doScan(this)">🔄 重新扫描</button></div><div class="mgrid" id="mgrid"></div>';
  loadMedia();
}
function loadMedia() {
  var q = ($('mq') && $('mq').value) || '';
  var g = $('mgrid');
  g.innerHTML = '<div class="empty"><span class="spin"></span> 加载中…</div>';
  api('/api/media?q=' + encodeURIComponent(q) + '&limit=200').then(function(list){
    mediaList = list || [];
    if (!mediaList.length) { g.innerHTML = '<div class="empty">没有媒体，先点「扫描」建立索引</div>'; return; }
    g.innerHTML = mediaList.map(function(m, i){
      var cov = m.has_cover ? '<img src="' + coverURL(m.id) + '" loading="lazy" alt="">' : '<div class="noimg">无封面</div>';
      return '<div class="mcard" onclick="openMedia(' + i + ')"><div class="cover">' + cov + '</div><div class="info"><div class="t" title="' + esc(m.title) + '">' + esc(m.title) + '</div><div class="m">' + esc(mediaMeta(m)) + '</div>' + subBadge(m.sub_status) + '</div></div>';
    }).join('');
  }).catch(function(e){ g.innerHTML = '<div class="empty">' + esc(e.message) + '</div>'; });
}
function openMedia(i) {
  var m = mediaList[i];
  var w = $('modalWrap'), md = $('modal');
  var cov = m.has_cover ? '<img src="' + coverURL(m.id) + '" style="width:120px;border-radius:8px;float:left;margin:0 14px 10px 0">' : '';
  md.innerHTML = '<button class="x" onclick="closeModal()">✕</button>' + cov +
    '<h3>' + esc(m.title) + '</h3>' +
    '<div class="note">' + esc(mediaMeta(m)) + '<br>' + esc(m.file_path) + '</div>' +
    '<div style="clear:both;margin-top:12px;display:flex;gap:8px;flex-wrap:wrap">' +
    '<button class="btn" id="mSearchBtn" onclick="searchMedia(\'' + m.id + '\')">🔍 搜索字幕</button>' +
    '<button class="btn ghost" onclick="downloadBest(\'' + m.id + '\')">⬇ 下载最佳匹配</button></div>' +
    '<div id="mResult" style="margin-top:14px"></div>';
  w.classList.add('on');
}
function closeModal() { $('modalWrap').classList.remove('on'); }
$('modalWrap') && document.addEventListener('click', function(e){
  if (e.target && e.target.id === 'modalWrap') closeModal();
});

var _hitReg = [];
function hitHTML(hit, mediaID) {
  var lang = hit.lang === 'zh-Hant' ? '繁体' : (hit.lang === 'zh-Hans' ? '简体' : esc(hit.lang));
  var idx = _hitReg.length;
  _hitReg.push({source: hit.source, ref_id: hit.ref_id, name: hit.name, media_id: mediaID || ''});
  return '<div class="hit"><div class="nm"><div class="fn">' + esc(hit.name) + '</div>' +
    '<div class="dt">' + esc(hit.detail || '') + (hit.votes ? ' · 👍' + hit.votes : '') + '</div></div>' +
    '<span class="tag">' + lang + '</span><span class="tag">' + esc(hit.format) + '</span>' +
    '<button class="btn small" onclick="dlHit(' + idx + ',this)">下载</button></div>';
}
function groupsHTML(groups, mediaID) {
  if (!groups || !groups.length) return '<div class="empty">没有启用的字幕源</div>';
  return groups.map(function(g){
    var body;
    if (g.error) body = '<div class="hit"><div class="nm"><div class="dt">搜索失败：' + esc(g.error) + '</div></div></div>';
    else if (!g.hits || !g.hits.length) body = '<div class="hit"><div class="nm"><div class="dt">无结果</div></div></div>';
    else body = g.hits.map(function(h){ return hitHTML(h, mediaID); }).join('');
    return '<div class="srcgroup"><div class="hd"><span>' + esc(g.source) + '</span>' +
      (g.cached ? '<span class="tag cached">缓存</span>' : '<span class="tag">实时</span>') + '</div>' + body + '</div>';
  }).join('');
}
function searchMedia(id) {
  var box = $('mResult');
  box.innerHTML = '<div class="empty"><span class="spin"></span> 聚合搜索中（6 个字幕源）…</div>';
  api('/api/search-media', {method:'POST', body:{id:id}}).then(function(r){
    if (!r.ok) { box.innerHTML = '<div class="empty">' + esc(r.detail || '搜索失败') + '</div>'; return; }
    box.innerHTML = '<div class="note" style="margin-bottom:8px">关键词：<code>' + esc(r.result.keyword) + '</code></div>' + groupsHTML(r.result.groups, id);
  }).catch(function(e){ box.innerHTML = '<div class="empty">' + esc(e.message) + '</div>'; });
}
function dlHit(idx, btn) {
  var h = _hitReg[idx];
  if (!h) return;
  btn.disabled = true; btn.textContent = '下载中…';
  api('/api/download', {method:'POST', body:{source:h.source, ref_id:h.ref_id, media_id:h.media_id, name:h.name}}).then(function(r){
    if (r.ok) { btn.textContent = '✅ 已保存'; if (h.media_id) setTimeout(loadMedia, 800); }
    else { btn.disabled = false; btn.textContent = '下载'; alert('失败：' + (r.detail || '')); }
  }).catch(function(e){ btn.disabled = false; btn.textContent = '下载'; alert(e.message); });
}
function downloadBest(id) {
  var box = $('mResult');
  box.innerHTML = '<div class="empty"><span class="spin"></span> 搜索并下载最佳匹配…</div>';
  api('/api/download-best', {method:'POST', body:{id:id}}).then(function(r){
    if (r.ok) { box.innerHTML = '<div class="empty">✅ 已保存：' + esc(r.save_path) + '</div>'; setTimeout(loadMedia, 800); }
    else box.innerHTML = '<div class="empty">失败：' + esc(r.detail || '') + '</div>';
  }).catch(function(e){ box.innerHTML = '<div class="empty">' + esc(e.message) + '</div>'; });
}

/* ---------- 字幕页 ---------- */
function renderSubPage() {
  var el = $('page-subtitles');
  el.innerHTML =
    '<div class="sec"><h3>手动聚合搜索</h3>' +
    '<div class="toolbar"><input type="text" id="kw" placeholder="输入标题或番号，如 SONE-028" onkeydown="if(event.key===\'Enter\')kwSearch()">' +
    '<button class="btn" onclick="kwSearch()">🔍 搜索</button></div>' +
    '<div class="note">搜索结果会缓存；下载的字幕保存在设置的「字幕目录」（未设置则存到数据目录 downloads/ 下）。</div>' +
    '<div id="kwResult" style="margin-top:12px"></div></div>' +
    '<div class="sec"><h3>下载历史</h3><div id="histList"><span class="spin"></span> 加载中…</div></div>';
  loadHistory();
}
function kwSearch() {
  var kw = $('kw').value.trim();
  if (!kw) return;
  var box = $('kwResult');
  box.innerHTML = '<div class="empty"><span class="spin"></span> 聚合搜索中…</div>';
  api('/api/search', {method:'POST', body:{keyword:kw}}).then(function(r){
    box.innerHTML = '<div class="note" style="margin-bottom:8px">关键词：<code>' + esc(r.keyword) + '</code></div>' + groupsHTML(r.groups, '');
  }).catch(function(e){ box.innerHTML = '<div class="empty">' + esc(e.message) + '</div>'; });
}
function loadHistory() {
  api('/api/history?limit=100').then(function(list){
    var t = $('histList');
    if (!list || !list.length) { t.innerHTML = '<div class="empty">暂无下载记录</div>'; return; }
    var rows = list.map(function(r){
      var st2 = r.status === 'ok' ? '<span class="badge ok">成功</span>' : '<span class="badge failed">失败</span>';
      return '<tr><td>' + esc(r.media_title || r.filename) + '</td><td>' + esc(r.source) + '</td>' +
        '<td title="' + esc(r.save_path || '') + '">' + esc(shortPath(r.save_path)) + '</td>' +
        '<td>' + st2 + '</td><td>' + esc(r.created_at) + '</td></tr>';
    }).join('');
    t.innerHTML = '<table><tr><th>标题</th><th>来源</th><th>保存路径</th><th>状态</th><th>时间</th></tr>' + rows + '</table>';
  });
}
function shortPath(p) {
  if (!p) return '—';
  var parts = p.split('/');
  return parts.length > 3 ? '…/' + parts.slice(-3).join('/') : p;
}

/* ---------- 设置页 ---------- */
var S = null;
function renderSettings() {
  var el = $('page-settings');
  el.innerHTML = '<div class="empty"><span class="spin"></span> 加载中…</div>';
  api('/api/settings').then(function(s){
    S = s;
    var h = '<div class="sec"><h3>字幕源</h3>';
    s.sources.forEach(function(src){
      var on = s.toggles[src.name] !== false;
      h += '<div class="row"><label><b>' + esc(src.name) + '</b>' + (src.enabled ? '' : ' <span class="tag">缺凭证</span>') + '</label>' +
        '<label class="switch"><input type="checkbox" data-src="' + esc(src.name) + '"' + (on ? ' checked' : '') + '><span class="sl"></span></label>' +
        '<span class="note">' + (on ? '启用' : '停用') + '</span></div>';
      if (src.name === 'assrt') h += '<div class="row"><label>ASSRT Token</label><div class="grow"><input type="text" id="f_assrt" value="' + esc(s.assrtToken) + '" placeholder="ASSRT API token"></div></div>';
      if (src.name === 'opensubtitles') {
        h += '<div class="row"><label>API Key</label><div class="grow"><input type="text" id="f_oskey" value="' + esc(s.osApiKey) + '"></div></div>';
        h += '<div class="row"><label>用户名</label><div class="grow"><input type="text" id="f_osuser" value="' + esc(s.osUser) + '"></div></div>';
        h += '<div class="row"><label>密码</label><div class="grow"><input type="password" id="f_ospass" placeholder="' + (s.hasOsPass ? '已保存（留空不改）' : '未设置') + '"></div></div>';
      }
      if (src.name === 'subdl') h += '<div class="row"><label>SubDL API Key</label><div class="grow"><input type="text" id="f_subdl" value="' + esc(s.subdlKey) + '"></div></div>';
    });
    h += '<div class="note">subhd / xunlei 免 key；subtitlecat 为成人番号字幕源（仅番号查询时触发）。</div></div>';

    h += '<div class="sec"><h3>标题识别正则</h3><div id="ruleList"></div>' +
      '<div class="row"><div class="grow"><input type="text" id="ruleName" placeholder="规则名称（可选）"></div></div>' +
      '<div class="row"><div class="grow"><input type="text" id="rulePattern" placeholder="正则，如 ^(?P<title>.+?)\\.S(?P<season>\\d+)E(?P<episode>\\d+)"></div>' +
      '<button class="btn small" onclick="addRule()">＋ 添加</button></div>' +
      '<div class="row"><div class="grow"><input type="text" id="testFn" placeholder="测试文件名，如 The.Matrix.1999.1080p.mkv"></div>' +
      '<button class="btn small ghost" onclick="testRule()">测试</button></div>' +
      '<div id="testOut" class="note"></div><div class="hint" id="ruleHint"></div></div>';

    h += '<div class="sec"><h3>通用</h3>' +
      '<div class="row"><label>扫描周期（分钟）</label><div class="grow"><input type="number" id="f_interval" value="' + s.scanInterval + '" min="5"></div></div>' +
      '<div class="row"><label>目标语言</label><div class="grow"><select id="f_lang"><option value="zh-Hans"' + (s.targetLang === 'zh-Hans' ? ' selected' : '') + '>简体中文</option><option value="zh-Hant"' + (s.targetLang === 'zh-Hant' ? ' selected' : '') + '>繁体中文</option></select></div></div>' +
      '<div class="row"><label>字幕目录</label><div class="grow"><input type="text" id="f_subdir" value="' + esc(s.subtitleDir) + '" placeholder="留空=视频同目录"></div></div>' +
      '<div class="row"><label>FlareSolverr</label><div class="grow"><input type="text" id="f_flare" value="' + esc(s.flaresolverr) + '" placeholder="http://host:8191（可选，用于过 Cloudflare）"></div></div>' +
      '<div class="note">媒体目录：' + s.mediaDirs.map(esc).join('；') + '（通过环境变量配置）</div></div>';

    h += '<div class="sec"><h3>API 令牌</h3>' +
      '<div class="row"><div class="grow"><input type="text" id="f_token" value="' + esc(s.apiToken) + '" readonly></div>' +
      '<button class="btn small ghost" onclick="regenToken()">重新生成</button></div>' +
      '<div class="note">调用 <code>/api/*</code> 时带 <code>X-API-Token</code> 请求头。</div></div>';

    h += '<div style="margin-bottom:30px"><button class="btn" onclick="saveSettings(this)">💾 保存设置</button></div>';
    el.innerHTML = h;
    loadRules();
  }).catch(function(e){ el.innerHTML = '<div class="empty">' + esc(e.message) + '</div>'; });
}
function saveSettings(btn) {
  btn.disabled = true;
  var toggles = {};
  document.querySelectorAll('input[data-src]').forEach(function(c){ toggles[c.getAttribute('data-src')] = c.checked; });
  var body = {
    toggles: toggles,
    assrtToken: $('f_assrt') ? $('f_assrt').value : '',
    osApiKey: $('f_oskey') ? $('f_oskey').value : '',
    osUser: $('f_osuser') ? $('f_osuser').value : '',
    osPass: $('f_ospass') ? $('f_ospass').value : '',
    subdlKey: $('f_subdl') ? $('f_subdl').value : '',
    scanInterval: parseInt($('f_interval').value, 10) || 60,
    targetLang: $('f_lang').value,
    subtitleDir: $('f_subdir').value.trim(),
    flaresolverr: $('f_flare').value.trim()
  };
  api('/api/settings', {method:'POST', body:body}).then(function(r){
    btn.disabled = false;
    alert(r.ok ? '✅ 已保存并热重载' : '保存失败');
  }).catch(function(e){ btn.disabled = false; alert(e.message); });
}
function regenToken() {
  if (!confirm('重新生成 API 令牌？旧令牌立即失效。')) return;
  api('/api/token/regenerate', {method:'POST'}).then(function(r){
    if (r.ok) { $('f_token').value = r.token; API_TOKEN = r.token; }
  });
}
function loadRules() {
  api('/api/rules').then(function(r){
    var box = $('ruleList');
    var list = r.rules || [];
    if (!list.length) box.innerHTML = '<div class="note">暂无自定义规则（内置规则仍生效）。</div>';
    else box.innerHTML = list.map(function(rl){
      return '<div class="rule"><div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">' +
        '<label class="switch"><input type="checkbox" data-rule="' + rl.id + '"' + (rl.enabled ? ' checked' : '') + ' onchange="toggleRule(' + rl.id + ',this.checked)"><span class="sl"></span></label>' +
        '<b>' + esc(rl.name || ('规则#' + rl.id)) + '</b>' +
        '<button class="btn small ghost" onclick="delRule(' + rl.id + ')">删除</button></div>' +
        '<div class="rp">' + esc(rl.pattern) + '</div></div>';
    }).join('');
    $('ruleHint').innerHTML = '命名分组：<code>(?P&lt;title&gt;…)</code> <code>(?P&lt;year&gt;…)</code> <code>(?P&lt;season&gt;…)</code> <code>(?P&lt;episode&gt;…)</code><br>内置规则：<br>' + r.defaults.map(function(d){ return '<code>' + esc(d) + '</code>'; }).join('<br>');
  });
}
function addRule() {
  var p = $('rulePattern').value.trim();
  if (!p) { alert('请输入正则'); return; }
  api('/api/rules', {method:'POST', body:{name:$('ruleName').value.trim(), pattern:p}}).then(function(r){
    if (r.ok) { $('ruleName').value = ''; $('rulePattern').value = ''; loadRules(); }
    else alert('添加失败');
  }).catch(function(e){ alert(e.message); });
}
function toggleRule(id, en) {
  var rl = null;
  api('/api/rules').then(function(r){
    (r.rules || []).forEach(function(x){ if (x.id === id) rl = x; });
    if (!rl) return;
    api('/api/rules/' + id, {method:'PUT', body:{name:rl.name, pattern:rl.pattern, ord:rl.ord, enabled:en}});
  });
}
function delRule(id) {
  if (!confirm('删除这条规则？')) return;
  api('/api/rules/' + id, {method:'DELETE'}).then(function(){ loadRules(); });
}
function testRule() {
  var p = $('rulePattern').value.trim();
  var fn = $('testFn').value.trim();
  if (!p || !fn) { alert('请填写正则和测试文件名'); return; }
  api('/api/rules/test', {method:'POST', body:{pattern:p, filename:fn}}).then(function(r){
    $('testOut').innerHTML = r.ok
      ? '✅ 标题：<b>' + esc(r.title) + '</b>' + (r.year ? ' · 年份 ' + r.year : '') + (r.season ? ' · S' + r.season + 'E' + r.episode : '')
      : '❌ ' + esc(r.detail);
  });
}

/* ---------- 启动 ---------- */
route();
</script>
</div>
</body>
</html>`
