package api

// The admin UI is server-rendered so it works in any phone browser without a
// build step. Keys are shown in plaintext and every key can be copied by tap.
const adminHead = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<meta name="theme-color" content="#6b4fa0">
<title>CoupleLink 管理</title>
<style>
:root{color-scheme:light;--bg:#f7f5fb;--card:#fff;--ink:#241b2f;--sub:#6d6480;--line:#ece7f3;--brand:#6b4fa0}
*{box-sizing:border-box}
body{margin:0;padding:16px;background:var(--bg);color:var(--ink);font:15px/1.5 system-ui,-apple-system,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif}
h1{font-size:20px;margin:0 0 2px}
h2{font-size:15px;margin:0 0 10px}
p.sub{margin:2px 0;color:var(--sub);font-size:13px}
a{color:var(--brand)}
.card{background:var(--card);border-radius:14px;padding:14px;margin:12px 0;box-shadow:0 1px 3px rgba(40,20,70,.08)}
.top{display:flex;gap:12px;flex-wrap:wrap;align-items:flex-start;justify-content:space-between}
nav{display:flex;gap:8px;flex-wrap:wrap}
.btn,button{font:inherit;border:0;border-radius:10px;padding:9px 14px;background:var(--brand);color:#fff;cursor:pointer;text-decoration:none;display:inline-block;text-align:center}
button.ghost,.btn.ghost{background:#efe9f8;color:#4a3a63}
button.danger{background:#f6e6e9;color:#a3263c}
input[type=text],input[type=password],input[type=file]{width:100%;padding:11px 12px;border:1px solid #d9d2e6;border-radius:10px;font:inherit;background:#fff}
label{display:block;margin:10px 0 0;font-size:13px;color:var(--sub)}
label input{margin-top:4px}
label.radio{display:flex;gap:8px;align-items:center;color:var(--ink);font-size:14px;margin:8px 0}
label.radio input{width:auto;margin:0}
.row{display:flex;gap:8px;flex-wrap:wrap}
.row input[type=text]{flex:1 1 200px;width:auto}
.col{display:flex;flex-direction:column;gap:6px}
.banner{background:#effaf3;border:1px solid #b8e3c8;border-radius:12px;padding:10px 12px;margin:12px 0;font-size:14px}
.err{color:#a3263c;font-size:13px;margin:8px 0 0}
.group{display:flex;gap:14px;flex-wrap:wrap;justify-content:space-between;background:var(--card);border-radius:14px;padding:14px;margin:10px 0;box-shadow:0 1px 3px rgba(40,20,70,.08)}
.grow{flex:1 1 320px;min-width:0}
.gside{display:flex;flex-direction:column;gap:8px;align-items:flex-end;font-size:12px;color:var(--sub)}
.gtitle{font-weight:600;display:flex;gap:8px;align-items:center;flex-wrap:wrap;margin-bottom:8px}
.badge{font-size:12px;padding:2px 8px;border-radius:999px;background:#efe9f8;color:#4a3a63;font-weight:500}
.badge.ok{background:#e2f6ea;color:#1a7f4b}.badge.warn{background:#fdf1de;color:#a4600a}.badge.off{background:#f0edf5;color:#8a8296}
.key{display:flex;gap:10px;align-items:center;flex-wrap:wrap;background:#f7f3ff;border:1px dashed #c9b8e6;color:var(--ink);padding:10px 12px;border-radius:10px;width:100%;text-align:left}
.key code{font-size:15px;letter-spacing:.02em;word-break:break-all}
.key .hint{font-size:12px;color:var(--sub)}
.key[disabled]{opacity:.7;cursor:default}
.key.link{background:#fdf9f0;border-color:#e2d4ae}
.key.link code{font-size:13px}
.mwho{display:inline-flex;align-items:baseline;gap:4px;margin-right:10px}.mwho b{font-weight:650}.mwho i{font-style:normal;font-size:10px;color:#a08494;text-transform:uppercase}
.gactions{display:flex;gap:8px;flex-wrap:wrap;margin-top:10px;align-items:center}
.fileform{display:flex;gap:6px;flex-wrap:wrap;align-items:center}
.fileform input[type=file]{width:auto;max-width:220px;padding:6px;font-size:12px}
.btn.small,button.small{padding:6px 10px;font-size:12px}
.group.binned{background:#fbf9fe}
dl{margin:10px 0 0;display:grid;gap:4px}
dl div{display:flex;gap:8px;font-size:13px}
dt{color:var(--sub);min-width:64px;margin:0}
dd{margin:0;word-break:break-word}
.narrow{max-width:420px;margin:6vh auto 0}
#toast{position:fixed;left:50%;bottom:24px;transform:translateX(-50%);background:#241b2f;color:#fff;padding:10px 16px;border-radius:999px;font-size:13px;max-width:90vw;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
@media (max-width:640px){body{padding:12px}.gside{align-items:flex-start;flex-direction:row;justify-content:space-between;width:100%}.narrow{margin-top:2vh}}
</style></head><body>
`

const adminTail = `
<div id="toast" hidden></div>
<script>
(function () {
  var toast = document.getElementById('toast');
  var timer;
  function show(message) {
    if (!toast) return;
    toast.textContent = message;
    toast.hidden = false;
    clearTimeout(timer);
    timer = setTimeout(function () { toast.hidden = true; }, 1800);
  }
  async function copy(text) {
    try { await navigator.clipboard.writeText(text); return true; } catch (error) { /* fall back below */ }
    try {
      var area = document.createElement('textarea');
      area.value = text;
      area.style.position = 'fixed';
      area.style.opacity = '0';
      document.body.appendChild(area);
      area.select();
      var ok = document.execCommand('copy');
      area.remove();
      return ok;
    } catch (error) { return false; }
  }
  document.addEventListener('click', function (event) {
    var target = event.target.closest('[data-copy]');
    if (!target) return;
    var text = target.getAttribute('data-copy') || '';
    if (!text) return;
    event.preventDefault();
    copy(text).then(function (ok) { show(ok ? '已复制：' + text : '复制失败，请长按选择'); });
  });
})();
</script>
</body></html>`

const adminLoginBody = `
<main class="narrow">
  <h1>CoupleLink 管理</h1>
  <p class="sub">使用管理账号登录，局域网内可直接打开。</p>
  <form class="card" method="post" action="/admin/login">
    <label>账号<input type="text" name="username" value="admin" autocomplete="username" required></label>
    <label>密码<input type="password" name="password" autocomplete="current-password" required autofocus></label>
    {{if .Error}}<p class="err">账号或密码不正确</p>{{end}}
    <p></p>
    <button class="primary" type="submit" style="width:100%">登录</button>
  </form>
  <p class="sub">初始密码在服务端启动时的控制台里显示，首次登录后必须修改。</p>
  <p class="sub"><a href="/">← 返回情侣端</a></p>
</main>`

const adminPasswordBody = `
<main class="narrow">
  <h1>修改账号密码</h1>
  <p class="sub">{{if .MustChange}}首次登录必须修改账号和初始密码。{{else}}账号和密码都可以在这里改。{{end}}</p>
  <form class="card" method="post" action="/admin/password">
    <label>账号<input type="text" name="username" value="{{.Username}}" autocomplete="username" minlength="3" maxlength="32" pattern="[a-z0-9_-]{3,32}" required></label>
    <label>当前密码<input type="password" name="current" autocomplete="current-password" required></label>
    <label>新密码<input type="password" name="password" autocomplete="new-password" minlength="8" required></label>
    <label>确认新密码<input type="password" name="confirm" autocomplete="new-password" minlength="8" required></label>
    {{if eq .Error "current"}}<p class="err">当前密码不正确</p>{{end}}
    {{if eq .Error "confirm"}}<p class="err">两次输入的新密码不一致</p>{{end}}
    {{if eq .Error "weak"}}<p class="err">新密码需 8 位以上，且不能包含控制字符</p>{{end}}
    {{if eq .Error "username"}}<p class="err">账号需为 3–32 位小写字母、数字、下划线或短横线</p>{{end}}
    {{if eq .Error "taken"}}<p class="err">这个账号已被占用</p>{{end}}
    <p></p>
    <button class="primary" type="submit" style="width:100%">保存新密码</button>
  </form>
</main>`

const adminPageBody = `
<header class="top">
  <div>
    <h1>CoupleLink 管理</h1>
    <p class="sub">版本 {{.Version}} · {{.Now}} · 账号 {{.Username}}</p>
  </div>
  <nav>
    <a class="btn" href="/admin/export">导出数据</a>
    <a class="btn ghost" href="/admin/password">改密码</a>
    <form method="post" action="/admin/logout"><button class="ghost" type="submit">退出</button></form>
  </nav>
</header>
{{if .Notice}}<div class="banner">{{.Notice}}</div>{{end}}
<section class="card">
  <h2>最新客户端下载</h2>
  {{if .HasDownload}}
  <p class="sub">版本 {{.DownloadVersion}} · {{.DownloadSize}}，把链接发给对方就能直接安装。</p>
  <button class="key" type="button" data-copy="{{.DownloadURL}}"><code>{{.DownloadURL}}</code><span class="hint">点击复制下载链接</span></button>
  <p><a class="btn" href="{{.DownloadURL}}">下载 APK</a></p>
  {{if ne .DownloadURL .LocalURL}}<p class="sub">局域网：<button class="key" type="button" data-copy="{{.LocalURL}}"><code>{{.LocalURL}}</code></button></p>{{end}}
  {{else}}
  <p class="sub">下载目录里还没有客户端安装包。</p>
  {{end}}
</section>

<section class="card">
  <h2>新建密钥</h2>
  <form class="row" method="post" action="/admin/keys">
    <input type="text" name="label" maxlength="40" placeholder="备注，例如 阿明和小雨">
    <button type="submit">生成密钥</button>
  </form>
  <p class="sub">每个组两把密钥：A、B 各一把，用哪把登录就绑定哪位成员；老组的共享密钥仍按第一台/第二台设备自动分配。</p>
</section>

<section class="card">
  <h2>数据备份</h2>
  <p class="sub">导出包含组信息、密钥、备注和全部聊天记录，浏览器直接下载。</p>
  <p><a class="btn" href="/admin/export">导出全部数据（JSON）</a></p>
  <form class="col" method="post" action="/admin/import" enctype="multipart/form-data">
    <input type="file" name="file" accept=".json,application/json" required>
    <label class="radio"><input type="radio" name="mode" value="merge" checked> 合并：保留现有数据，只补缺失的组和消息</label>
    <label class="radio"><input type="radio" name="mode" value="overwrite"> 覆盖：清空现有组、密钥和聊天后导入</label>
    <button type="submit">导入备份</button>
  </form>
</section>

<section>
  {{range .Groups}}
  <article class="group">
    <div class="grow">
      <div class="gtitle">{{if .Label}}{{.Label}}{{else}}未命名组{{end}}<span class="badge {{.BadgeClass}}">{{.Bound}}</span></div>
      {{if .HasKey}}
      <button class="key" type="button" data-copy="{{.Key}}"><code>A · {{.Key}}</code><span class="hint">成员 A 的密钥，点击复制</span></button>
      <button class="key link" type="button" data-copy="{{$.Base}}-{{.Key}}"><code>{{$.Base}}-{{.Key}}</code><span class="hint">成员 A 的登录链接（粘贴到 App）</span></button>
      {{else}}
      <button class="key" type="button" disabled><code>{{.Hint}}…</code><span class="hint">旧密钥不可显示，请重新生成</span></button>
      {{end}}
      {{if .HasKeyB}}
      <button class="key" type="button" data-copy="{{.KeyB}}"><code>B · {{.KeyB}}</code><span class="hint">成员 B 的密钥，点击复制</span></button>
      <button class="key link" type="button" data-copy="{{$.Base}}-{{.KeyB}}"><code>{{$.Base}}-{{.KeyB}}</code><span class="hint">成员 B 的登录链接（粘贴到 App）</span></button>
      {{end}}
      <dl>
        <div><dt>成员</dt><dd><span class="mwho"><b>{{.MemberA}}</b><i>A</i></span> <span class="mwho"><b>{{.MemberB}}</b><i>B</i></span></dd></div>
        <div><dt>备注</dt><dd>{{if .RemarkA}}{{.RemarkA}}{{else}}—{{end}} / {{if .RemarkB}}{{.RemarkB}}{{else}}—{{end}}</dd></div>
        <div><dt>设备</dt><dd>{{.DevicesA}} / {{.DevicesB}}</dd></div>
        <div><dt>最近登录</dt><dd>{{.LastUsed}}</dd></div>
      </dl>
      {{if and .HasKey (not .HasKeyB)}}
      <p class="sub">这是老组的共享密钥：补发后 A、B 各一把，用哪把登录就绑定哪位成员。</p>
      {{end}}
      <div class="gactions">
        {{if and .HasKey (not .HasKeyB)}}
        <form method="post" action="/admin/groups/{{.Couple}}/issue-key"><button class="ghost small" type="submit">补发 B 密钥</button></form>
        {{end}}
        <a class="btn ghost small" href="/admin/groups/{{.Couple}}/export">导出该组（{{.Messages}} 条聊天）</a>
        <form class="fileform" method="post" action="/admin/groups/{{.Couple}}/import" enctype="multipart/form-data">
          <input type="file" name="file" accept=".json,application/json" required>
          <button class="ghost small" type="submit">导入聊天</button>
        </form>
      </div>
    </div>
    <div class="gside">
      <span>创建 {{.Created}}</span>
      <form method="post" action="/admin/keys/{{.KeyID}}/revoke"><button class="danger" type="submit">移入回收站</button></form>
    </div>
  </article>
  {{else}}
  <p class="sub">还没有密钥，点上面的按钮生成第一个。</p>
  {{end}}
</section>

<section class="card">
  <h2>回收站</h2>
  <p class="sub">吊销的组会先进这里，可以恢复；永久删除后该组的聊天记录无法找回。</p>
  {{range .Binned}}
  <article class="group binned">
    <div class="grow">
      <div class="gtitle">{{if .Label}}{{.Label}}{{else}}未命名组{{end}}<span class="badge off">回收站</span></div>
      {{if .HasKey}}
      <button class="key" type="button" data-copy="{{.Key}}"><code>{{.Key}}</code><span class="hint">点击复制密钥</span></button>
      {{else}}
      <button class="key" type="button" disabled><code>{{.Hint}}…</code><span class="hint">旧密钥不可显示</span></button>
      {{end}}
      <dl>
        <div><dt>成员</dt><dd><span class="mwho"><b>{{.MemberA}}</b><i>A</i></span> <span class="mwho"><b>{{.MemberB}}</b><i>B</i></span></dd></div>
        <div><dt>聊天</dt><dd>{{.Messages}} 条</dd></div>
      </dl>
    </div>
    <div class="gside">
      <form method="post" action="/admin/keys/{{.KeyID}}/restore"><button class="ghost" type="submit">恢复</button></form>
      <form method="post" action="/admin/keys/{{.KeyID}}/purge" onsubmit="return confirm('永久删除这个组和全部聊天记录？此操作不可恢复。')"><button class="danger" type="submit">永久删除</button></form>
    </div>
  </article>
  {{else}}
  <p class="sub">回收站是空的。</p>
  {{end}}
</section>
<p class="sub">吊销后该密钥立即失效；已登录设备会在下次会话过期后失效。<a href="/">返回情侣端</a></p>`
