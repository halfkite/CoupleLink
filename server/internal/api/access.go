package api

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Access keys replace username/password accounts: the admin page creates one key
// per couple group, and the same key signs in both phones of that group.
var accessKeyPattern = regexpKey()

func regexpKey() *simplePattern {
	return &simplePattern{prefix: "key", alphabet: "abcdefghijklmnopqrstuvwxyz234567", min: 16, max: 32}
}

// simplePattern avoids pulling a regexp into every request path.
type simplePattern struct {
	prefix   string
	alphabet string
	min, max int
}

func (p *simplePattern) Match(value string) bool {
	if !strings.HasPrefix(value, p.prefix) {
		return false
	}
	body := value[len(p.prefix):]
	if len(body) < p.min || len(body) > p.max {
		return false
	}
	for _, r := range body {
		if !strings.ContainsRune(p.alphabet, r) {
			return false
		}
	}
	return true
}

func newAccessKey() (string, error) {
	raw := make([]byte, 13)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	encoded := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw))
	if len(encoded) < 20 {
		return "", errors.New("short key")
	}
	return "key" + encoded[:20], nil
}

// createGroupAndKey provisions a two-person group and its single shared key.
func (s *Server) createGroupAndKey(ctx context.Context, label string) (string, string, error) {
	key, err := newAccessKey()
	if err != nil {
		return "", "", err
	}
	coupleID, err := identifier()
	if err != nil {
		return "", "", err
	}
	keyID, err := identifier()
	if err != nil {
		return "", "", err
	}
	userA, err := identifier()
	if err != nil {
		return "", "", err
	}
	userB, err := identifier()
	if err != nil {
		return "", "", err
	}
	unusable, err := randomToken()
	if err != nil {
		return "", "", err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)
	for index, id := range []string{userA, userB} {
		username := "group-" + strings.ReplaceAll(id, "-", "")[:16]
		display := fmt.Sprintf("成员 %s", []string{"A", "B"}[index])
		if _, err = tx.Exec(ctx, `INSERT INTO users(id,username,display_name,password_hash) VALUES($1,$2,$3,$4)`, id, username, display, tokenHash(unusable)); err != nil {
			return "", "", err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO sync_heads(user_id) VALUES($1)`, id); err != nil {
			return "", "", err
		}
	}
	start := time.Now().In(chinaZone()).Format("2006-01-02")
	if _, err = tx.Exec(ctx, `INSERT INTO couples(id,user_a,user_b,start_at,timezone,status) VALUES($1,$2,$3,$4,'Asia/Shanghai','active')`, coupleID, userA, userB, start); err != nil {
		return "", "", err
	}
	for _, id := range []string{userA, userB} {
		if _, err = tx.Exec(ctx, `INSERT INTO couple_members(user_id,couple_id) VALUES($1,$2)`, id, coupleID); err != nil {
			return "", "", err
		}
	}
	hint := key
	if len(hint) > 10 {
		hint = hint[:10]
	}
	if _, err = tx.Exec(ctx, `INSERT INTO access_keys(id,key_hash,key_hint,couple_id,label) VALUES($1,$2,$3,$4,$5)`, keyID, tokenHash(key), hint, coupleID, label); err != nil {
		return "", "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return key, coupleID, nil
}

func chinaZone() *time.Location {
	zone, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return zone
}

type keyLoginInput struct {
	Key         string `json:"key"`
	DeviceID    string `json:"device_id"`
	DeviceLabel string `json:"device_label"`
	Platform    string `json:"platform"`
}

// keyLogin signs a device in with the group's key and assigns it to the group
// member that has fewer active devices.
func (s *Server) keyLogin(w http.ResponseWriter, r *http.Request) error {
	var input keyLoginInput
	if err := decode(w, r, &input); err != nil {
		return err
	}
	input.Key = strings.ToLower(strings.TrimSpace(input.Key))
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !s.limiter.allow(host) {
		return fail(429, "rate_limited")
	}
	if !accessKeyPattern.Match(input.Key) || !uuidPattern.MatchString(input.DeviceID) || !validText(input.DeviceLabel, 64) || !validText(input.Platform, 24) {
		return fail(400, "invalid_input")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var keyID, coupleID, userA, userB string
	err = tx.QueryRow(r.Context(), `SELECT k.id::text,k.couple_id::text,c.user_a::text,c.user_b::text
  FROM access_keys k JOIN couples c ON c.id=k.couple_id
  WHERE k.key_hash=$1 AND k.revoked_at IS NULL`, tokenHash(input.Key)).Scan(&keyID, &coupleID, &userA, &userB)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(401, "invalid_key")
	}
	if err != nil {
		return err
	}
	var countA, countB int
	var deviceA, deviceB bool
	if err = tx.QueryRow(r.Context(), `SELECT
  (SELECT count(*) FROM devices WHERE user_id=$1 AND revoked_at IS NULL),
  (SELECT count(*) FROM devices WHERE user_id=$2 AND revoked_at IS NULL),
  EXISTS(SELECT 1 FROM devices WHERE id=$3 AND user_id=$1),
  EXISTS(SELECT 1 FROM devices WHERE id=$3 AND user_id=$2)`, userA, userB, input.DeviceID).Scan(&countA, &countB, &deviceA, &deviceB); err != nil {
		return err
	}
	member := userA
	switch {
	case deviceA:
		member = userA
	case deviceB:
		member = userB
	case countB < countA:
		member = userB
	}
	if err = lockUsers(r.Context(), tx, member); err != nil {
		return err
	}
	token, expires, err := s.newSession(r.Context(), tx, member, authInput{DeviceID: input.DeviceID, DeviceLabel: input.DeviceLabel, Platform: input.Platform})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `UPDATE access_keys SET last_used_at=now() WHERE id=$1`, keyID); err != nil {
		return err
	}
	var user User
	if err = tx.QueryRow(r.Context(), `SELECT id::text,username,display_name FROM users WHERE id=$1`, member).Scan(&user.ID, &user.Username, &user.DisplayName); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]any{"token": token, "expires_at": expires, "user": user, "couple_id": coupleID})
	return nil
}

func (s *Server) adminAllowed(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && (host == "127.0.0.1" || host == "::1" || host == "10.0.2.2") {
		return true
	}
	token := r.URL.Query().Get("t")
	if token == "" {
		token = r.Header.Get("X-Admin-Token")
	}
	return s.cfg.AdminToken != "" && secretEqual(token, s.cfg.AdminToken)
}

type adminRow struct {
	Couple      string
	KeyHint     string
	Label       string
	Created     string
	LastUsed    string
	MemberA     string
	MemberB     string
	DevicesA    int
	DevicesB    int
	Bound       string
	KeyID       string
	Revoked     bool
}

type adminView struct {
	Version string
	Now     string
	Created string
	Rows    []adminRow
	Token   string
	Listen  string
}

func (s *Server) adminPage(w http.ResponseWriter, r *http.Request) error {
	if !s.adminAllowed(r) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(403)
		_, _ = w.Write([]byte("需要从本机浏览器打开，或提供管理令牌：/admin?t=<令牌>\n"))
		return nil
	}
	rows, err := s.adminRows(r.Context())
	if err != nil {
		return err
	}
	view := adminView{Version: s.cfg.Version, Now: time.Now().In(chinaZone()).Format("2006-01-02 15:04:05"), Created: r.URL.Query().Get("created"), Rows: rows, Token: r.URL.Query().Get("t"), Listen: s.cfg.Version}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, err := template.New("admin").Parse(adminHTML)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, view)
}

func (s *Server) adminRows(ctx context.Context) ([]adminRow, error) {
	rows, err := s.db.Query(ctx, `SELECT c.id::text,k.id::text,k.key_hint,k.label,
  to_char(k.created_at,'YYYY-MM-DD HH24:MI'),COALESCE(to_char(k.last_used_at,'YYYY-MM-DD HH24:MI'),'—'),
  ua.display_name,ub.display_name,
  (SELECT count(*) FROM devices d WHERE d.user_id=c.user_a AND d.revoked_at IS NULL),
  (SELECT count(*) FROM devices d WHERE d.user_id=c.user_b AND d.revoked_at IS NULL),
  k.revoked_at IS NOT NULL
  FROM access_keys k JOIN couples c ON c.id=k.couple_id
  JOIN users ua ON ua.id=c.user_a JOIN users ub ON ub.id=c.user_b
  ORDER BY k.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []adminRow
	for rows.Next() {
		var row adminRow
		var revoked bool
		var lastUsed string
		if err := rows.Scan(&row.Couple, &row.KeyID, &row.KeyHint, &row.Label, &row.Created, &lastUsed, &row.MemberA, &row.MemberB, &row.DevicesA, &row.DevicesB, &revoked); err != nil {
			return nil, err
		}
		row.LastUsed = lastUsed
		row.Revoked = revoked
		switch {
		case revoked:
			row.Bound = "已吊销"
		case row.DevicesA > 0 && row.DevicesB > 0:
			row.Bound = "双方已登录"
		case row.DevicesA > 0 || row.DevicesB > 0:
			row.Bound = "已登录 1 人"
		default:
			row.Bound = "未登录"
		}
		list = append(list, row)
	}
	return list, rows.Err()
}

func (s *Server) adminCreateKey(w http.ResponseWriter, r *http.Request) error {
	if !s.adminAllowed(r) {
		write(w, 403, map[string]string{"code": "forbidden"})
		return nil
	}
	if err := r.ParseForm(); err != nil {
		write(w, 400, map[string]string{"code": "invalid_input"})
		return nil
	}
	label := strings.TrimSpace(r.FormValue("label"))
	if len([]rune(label)) > 40 {
		write(w, 400, map[string]string{"code": "invalid_input"})
		return nil
	}
	key, _, err := s.createGroupAndKey(r.Context(), label)
	if err != nil {
		return err
	}
	query := "/admin?created=" + key
	if token := r.URL.Query().Get("t"); token != "" {
		query += "&t=" + token
	}
	http.Redirect(w, r, query, http.StatusSeeOther)
	return nil
}

func (s *Server) adminRevokeKey(w http.ResponseWriter, r *http.Request) error {
	if !s.adminAllowed(r) {
		write(w, 403, map[string]string{"code": "forbidden"})
		return nil
	}
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		return fail(400, "invalid_input")
	}
	if _, err := s.db.Exec(r.Context(), `UPDATE access_keys SET revoked_at=now() WHERE id=$1 AND revoked_at IS NULL`, id); err != nil {
		return err
	}
	query := "/admin"
	if token := r.URL.Query().Get("t"); token != "" {
		query += "?t=" + token
	}
	http.Redirect(w, r, query, http.StatusSeeOther)
	return nil
}

const adminHTML = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>CoupleLink 管理</title>
<style>
:root{color-scheme:light dark}
body{font-family:system-ui,-apple-system,"Segoe UI",sans-serif;margin:0;padding:24px;background:#f7f5fb;color:#241b2f}
h1{font-size:20px;margin:0 0 4px}
p.sub{margin:0 0 20px;color:#6d6480;font-size:13px}
.card{background:#fff;border-radius:12px;padding:16px 18px;margin-bottom:16px;box-shadow:0 1px 3px rgba(40,20,70,.08)}
table{border-collapse:collapse;width:100%;font-size:13px}
th,td{text-align:left;padding:6px 8px;border-bottom:1px solid #ece7f3;vertical-align:top}
th{color:#6d6480;font-weight:600;font-size:12px;text-transform:uppercase;letter-spacing:.03em}
code{background:#f2eefb;padding:2px 6px;border-radius:6px;font-size:13px}
.ok{color:#1a7f4b}.warn{color:#a4600a}.off{color:#8a8296}
form.inline{display:flex;gap:8px;align-items:center;flex-wrap:wrap}
input[type=text]{padding:8px 10px;border:1px solid #d9d2e6;border-radius:8px;font-size:13px;min-width:180px}
button{padding:8px 14px;border:0;border-radius:8px;background:#6b4fa0;color:#fff;font-size:13px;cursor:pointer}
button.ghost{background:#efe9f8;color:#4a3a63}
.created{border:1px solid #b8e3c8;background:#effaf3}
a{color:#6b4fa0}
</style></head><body>
<h1>CoupleLink 管理</h1>
<p class="sub">版本 {{.Version}} · 服务器时间 {{.Now}} · 每个密钥对应一个情侣组</p>
{{if .Created}}<div class="card created"><strong>新密钥（只显示这一次）</strong><p><code>{{.Created}}</code></p>
<p class="sub">登录链接示例：<code>http://127.0.0.1:30710-{{.Created}}</code></p></div>{{end}}
<div class="card">
  <form class="inline" method="post" action="/admin/keys{{if .Token}}?t={{.Token}}{{end}}">
    <input type="text" name="label" placeholder="备注（可留空，例如 阿明和小雨）" maxlength="40">
    <button type="submit">生成密钥（新建一个组）</button>
  </form>
</div>
<div class="card">
<table>
<thead><tr><th>组</th><th>密钥</th><th>备注</th><th>成员</th><th>绑定状态</th><th>设备</th><th>最近登录</th><th>操作</th></tr></thead>
<tbody>
{{range .Rows}}<tr>
<td><code>{{.Couple}}</code></td>
<td><code>{{.KeyHint}}…</code></td>
<td>{{.Label}}</td>
<td>{{.MemberA}} / {{.MemberB}}</td>
<td>{{if eq .Bound "双方已登录"}}<span class="ok">{{.Bound}}</span>{{else if eq .Bound "已登录 1 人"}}<span class="warn">{{.Bound}}</span>{{else}}<span class="off">{{.Bound}}</span>{{end}}</td>
<td>{{.DevicesA}} / {{.DevicesB}}</td>
<td>{{.LastUsed}}</td>
<td>{{if .Revoked}}—{{else}}<form method="post" action="/admin/keys/{{.KeyID}}/revoke{{if $.Token}}?t={{$.Token}}{{end}}"><button class="ghost" type="submit">吊销</button></form>{{end}}</td>
</tr>{{else}}<tr><td colspan="8" class="off">还没有密钥，点上面的按钮生成第一个。</td></tr>{{end}}
</tbody></table>
</div>
<p class="sub">吊销后该密钥立即失效；已登录的设备在下次会话过期后失效。管理页默认只允许本机访问，局域网访问需带 <code>?t=管理令牌</code>。</p>
</body></html>`