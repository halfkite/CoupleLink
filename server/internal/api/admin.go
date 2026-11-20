package api

import (
	"context"
	"crypto/rand"
	"errors"
	"html/template"
	"os"
	"path/filepath"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	adminCookieName  = "cl_admin"
	adminSessionTTL  = 12 * time.Hour
	adminDefaultUser = "admin"
	adminPasswordMin = 8
	adminPasswordMax = 128
)

type adminSession struct {
	ID         string
	Username   string
	MustChange bool
}

// AdminCredentials is the management account as printed at startup and stored in
// the configured password file.
type AdminCredentials struct {
	Username  string
	Password  string
	Generated bool
}

// EnsureAdmin creates the default admin account on first start and keeps the
// plaintext password available: it is printed on every start and written to the
// configured password file, so it is never lost after the one-time banner.
// The plaintext exists only in this local database and that file.
func EnsureAdmin(ctx context.Context, db *pgxpool.Pool, override, passwordFile string) (AdminCredentials, error) {
	var username, hash, plain string
	err := db.QueryRow(ctx, `SELECT username,password_hash,password_plain FROM admin_users ORDER BY created_at LIMIT 1`).Scan(&username, &hash, &plain)
	if errors.Is(err, pgx.ErrNoRows) {
		password, err := chooseAdminPassword(override)
		if err != nil {
			return AdminCredentials{}, err
		}
		id, err := identifier()
		if err != nil {
			return AdminCredentials{}, err
		}
		encoded, err := hashPassword(password)
		if err != nil {
			return AdminCredentials{}, err
		}
		if _, err = db.Exec(ctx, `INSERT INTO admin_users(id,username,password_hash,password_plain,must_change) VALUES($1,$2,$3,$4,true)`,
			id, adminDefaultUser, encoded, password); err != nil {
			return AdminCredentials{}, err
		}
		return AdminCredentials{Username: adminDefaultUser, Password: password, Generated: true}, writeAdminFile(passwordFile, adminDefaultUser, password)
	}
	if err != nil {
		return AdminCredentials{}, err
	}
	password := plain
	rotated := false
	switch {
	case override != "" && !verifyPassword(hash, override):
		// An explicit COUPLELINK_ADMIN_PASSWORD always resets the account.
		password, rotated = override, true
	case plain == "":
		generated, err := initialAdminPassword()
		if err != nil {
			return AdminCredentials{}, err
		}
		password, rotated = generated, true
	}
	if rotated {
		encoded, err := hashPassword(password)
		if err != nil {
			return AdminCredentials{}, err
		}
		if _, err = db.Exec(ctx, `UPDATE admin_users SET password_hash=$1,password_plain=$2,must_change=true,updated_at=now() WHERE username=$3`,
			encoded, password, adminDefaultUser); err != nil {
			return AdminCredentials{}, err
		}
	}
	return AdminCredentials{Username: username, Password: password, Generated: rotated}, writeAdminFile(passwordFile, username, password)
}

func chooseAdminPassword(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	return initialAdminPassword()
}

// writeAdminFile keeps a readable copy of the management credentials next to the
// server's private files; the web password change rewrites it.
func writeAdminFile(path, username, password string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := "# CoupleLink 管理页账号：每次启动都会显示，网页修改密码后同步更新\nusername=" + username + "\npassword=" + password + "\n"
	return os.WriteFile(path, []byte(content), 0o600)
}

func initialAdminPassword() (string, error) {
	const alphabet = "abcdefghijkmnpqrstuvwxyz23456789"
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	out := make([]byte, len(raw))
	for index, value := range raw {
		out[index] = alphabet[int(value)%len(alphabet)]
	}
	return string(out), nil
}
func (s *Server) readAdminSession(r *http.Request) (adminSession, bool) {
	cookie, err := r.Cookie(adminCookieName)
	if err != nil || len(cookie.Value) != 43 {
		return adminSession{}, false
	}
	var session adminSession
	err = s.db.QueryRow(r.Context(), `SELECT a.id::text,a.username,a.must_change
  FROM admin_sessions x JOIN admin_users a ON a.id=x.admin_id
  WHERE x.token_hash=$1 AND x.expires_at>now()`, tokenHash(cookie.Value)).Scan(&session.ID, &session.Username, &session.MustChange)
	if err != nil {
		return adminSession{}, false
	}
	return session, true
}

func (s *Server) setAdminCookie(w http.ResponseWriter, r *http.Request, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   maxAge,
	})
}

// requireAdmin redirects anonymous visitors to the login page and forces the
// initial password change before anything else is reachable.
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request, allowMustChange bool) (adminSession, bool) {
	session, ok := s.readAdminSession(r)
	if !ok {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return adminSession{}, false
	}
	if session.MustChange && !allowMustChange {
		http.Redirect(w, r, "/admin/password", http.StatusSeeOther)
		return adminSession{}, false
	}
	return session, true
}

func (s *Server) adminLoginPage(w http.ResponseWriter, r *http.Request) error {
	if session, ok := s.readAdminSession(r); ok {
		if session.MustChange {
			http.Redirect(w, r, "/admin/password", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
		}
		return nil
	}
	view := struct{ Error bool }{Error: r.URL.Query().Get("error") == "1"}
	return renderAdmin(w, adminLoginBody, view)
}

func (s *Server) adminLoginSubmit(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/login?error=1", http.StatusSeeOther)
		return nil
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !s.limiter.allow("admin:" + host) {
		http.Redirect(w, r, "/admin/login?error=1", http.StatusSeeOther)
		return nil
	}
	username := strings.ToLower(strings.TrimSpace(r.FormValue("username")))
	if username == "" {
		username = adminDefaultUser
	}
	password := r.FormValue("password")
	var id, hash string
	var mustChange bool
	err := s.db.QueryRow(r.Context(), `SELECT id::text,password_hash,must_change FROM admin_users WHERE username=$1`, username).Scan(&id, &hash, &mustChange)
	if err != nil || !verifyPassword(hash, password) {
		http.Redirect(w, r, "/admin/login?error=1", http.StatusSeeOther)
		return nil
	}
	token, err := randomToken()
	if err != nil {
		return err
	}
	expires := time.Now().Add(adminSessionTTL)
	if _, err = s.db.Exec(r.Context(), `DELETE FROM admin_sessions WHERE expires_at<now()`); err != nil {
		return err
	}
	if _, err = s.db.Exec(r.Context(), `INSERT INTO admin_sessions(token_hash,admin_id,expires_at) VALUES($1,$2,$3)`, tokenHash(token), id, expires); err != nil {
		return err
	}
	s.setAdminCookie(w, r, token, int(adminSessionTTL.Seconds()))
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
	return nil
}

func (s *Server) adminLogout(w http.ResponseWriter, r *http.Request) error {
	if cookie, err := r.Cookie(adminCookieName); err == nil && cookie.Value != "" {
		if _, err = s.db.Exec(r.Context(), `DELETE FROM admin_sessions WHERE token_hash=$1`, tokenHash(cookie.Value)); err != nil {
			return err
		}
	}
	s.setAdminCookie(w, r, "", -1)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	return nil
}

func (s *Server) adminPasswordPage(w http.ResponseWriter, r *http.Request) error {
	if _, ok := s.requireAdmin(w, r, true); !ok {
		return nil
	}
	view := struct {
		Username   string
		MustChange bool
		Error      string
	}{Username: adminDefaultUser, MustChange: true, Error: r.URL.Query().Get("error")}
	if session, ok := s.readAdminSession(r); ok {
		view.MustChange = session.MustChange
		view.Username = session.Username
	}
	return renderAdmin(w, adminPasswordBody, view)
}

func (s *Server) adminPasswordSubmit(w http.ResponseWriter, r *http.Request) error {
	session, ok := s.requireAdmin(w, r, true)
	if !ok {
		return nil
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/password?error=form", http.StatusSeeOther)
		return nil
	}
	current := r.FormValue("current")
	next := r.FormValue("password")
	confirm := r.FormValue("confirm")
	username := strings.ToLower(strings.TrimSpace(r.FormValue("username")))
	if username == "" {
		username = session.Username
	}
	if !usernamePattern.MatchString(username) {
		http.Redirect(w, r, "/admin/password?error=username", http.StatusSeeOther)
		return nil
	}
	var hash string
	if err := s.db.QueryRow(r.Context(), `SELECT password_hash FROM admin_users WHERE id=$1`, session.ID).Scan(&hash); err != nil {
		return err
	}
	if username != session.Username {
		var taken bool
		if err := s.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM admin_users WHERE username=$1 AND id<>$2)`, username, session.ID).Scan(&taken); err != nil {
			return err
		}
		if taken {
			http.Redirect(w, r, "/admin/password?error=taken", http.StatusSeeOther)
			return nil
		}
	}
	switch {
	case !verifyPassword(hash, current):
		http.Redirect(w, r, "/admin/password?error=current", http.StatusSeeOther)
		return nil
	case next != confirm:
		http.Redirect(w, r, "/admin/password?error=confirm", http.StatusSeeOther)
		return nil
	case len([]rune(next)) < adminPasswordMin || len([]rune(next)) > adminPasswordMax || !validOptionalText(next):
		http.Redirect(w, r, "/admin/password?error=weak", http.StatusSeeOther)
		return nil
	}
	newHash, err := hashPassword(next)
	if err != nil {
		return err
	}
	if _, err = s.db.Exec(r.Context(), `UPDATE admin_users SET username=$1,password_hash=$2,password_plain=$3,must_change=false,updated_at=now() WHERE id=$4`, username, newHash, next, session.ID); err != nil {
		return err
	}
	if cookie, err := r.Cookie(adminCookieName); err == nil {
		if _, err = s.db.Exec(r.Context(), `DELETE FROM admin_sessions WHERE admin_id=$1 AND token_hash<>$2`, session.ID, tokenHash(cookie.Value)); err != nil {
			return err
		}
	}
	_ = writeAdminFile(s.cfg.AdminFile, username, next)
	http.Redirect(w, r, "/admin?changed=1", http.StatusSeeOther)
	return nil
}

type adminGroupView struct {
	Couple    string
	KeyID     string
	Key       string
	Hint      string
	HasKey    bool
	KeyB      string
	HintB     string
	HasKeyB   bool
	Label     string
	Created   string
	LastUsed  string
	Bound     string
	BadgeClass string
	MemberA   string
	MemberB   string
	GenderA   string
	GenderB   string
	RemarkA   string
	RemarkB   string
	DevicesA  int
	DevicesB  int
	Messages  int
	Revoked   bool
	Archived  bool
}

type adminPageView struct {
	Version  string
	Now      string
	Base     string
	Username string
	Notice   string
	Groups   []adminGroupView
	Binned   []adminGroupView

	HasDownload     bool
	DownloadVersion string
	DownloadURL     string
	LocalURL        string
	DownloadSize    string
}

func genderLabel(value string) string {
	switch value {
	case "male":
		return "男"
	case "female":
		return "女"
	}
	return "未选"
}

func (s *Server) adminPage(w http.ResponseWriter, r *http.Request) error {
	session, ok := s.requireAdmin(w, r, false)
	if !ok {
		return nil
	}
	groups, binned, err := s.adminGroups(r.Context())
	if err != nil {
		return err
	}
	notice := ""
	switch r.URL.Query().Get("changed") {
	case "1":
		notice = "账号和密码已更新。"
	}
	if created := r.URL.Query().Get("created"); created != "" {
		notice = "已生成新密钥，可在列表里随时复制。"
	}
	if imported := r.URL.Query().Get("imported"); imported != "" {
		notice = "导入完成：" + imported
	}
	if failed := r.URL.Query().Get("error"); failed != "" {
		notice = "操作失败：" + failed
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	view := adminPageView{
		Version:  s.cfg.Version,
		Now:      time.Now().In(chinaZone()).Format("2006-01-02 15:04:05"),
		Base:     scheme + "://" + r.Host,
		Username: session.Username,
		Notice:   notice,
		Groups:   groups,
		Binned:   binned,
	}
	if name, version, size, ok := s.latestAPK(); ok {
		view.HasDownload = true
		view.DownloadVersion = version
		view.LocalURL = view.Base + "/download/" + name
		view.DownloadURL = view.LocalURL
		if s.cfg.PublicURL != "" {
			view.DownloadURL = s.cfg.PublicURL + "/download/" + name
		}
		view.DownloadSize = humanSize(size)
	}
	return renderAdmin(w, adminPageBody, view)
}

func (s *Server) adminGroups(ctx context.Context) (active, binned []adminGroupView, err error) {
	rows, err := s.db.Query(ctx, `SELECT c.id::text,COALESCE(k.id::text,''),COALESCE(k.key_plain,''),COALESCE(k.key_hint,''),COALESCE(k.label,''),
  COALESCE(to_char(k.created_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(k.last_used_at,'YYYY-MM-DD HH24:MI'),'—'),
  ua.display_name,ua.gender,c.remark_a,ub.display_name,ub.gender,c.remark_b,
  (SELECT count(*) FROM devices d WHERE d.user_id=c.user_a AND d.revoked_at IS NULL),
  (SELECT count(*) FROM devices d WHERE d.user_id=c.user_b AND d.revoked_at IS NULL),
  (SELECT count(*) FROM messages m WHERE m.couple_id=c.id),
  COALESCE(k.revoked_at IS NOT NULL,false),c.archived_at IS NOT NULL,
  COALESCE(kb.key_plain,''),COALESCE(kb.key_hint,'')
  FROM couples c
  JOIN users ua ON ua.id=c.user_a JOIN users ub ON ub.id=c.user_b
  LEFT JOIN LATERAL (SELECT * FROM access_keys k2 WHERE k2.couple_id=c.id AND (k2.slot='a' OR k2.slot IS NULL)
    ORDER BY (k2.slot='a') DESC NULLS LAST, k2.created_at DESC LIMIT 1) k ON true
  LEFT JOIN LATERAL (SELECT * FROM access_keys k3 WHERE k3.couple_id=c.id AND k3.slot='b'
    ORDER BY k3.created_at DESC LIMIT 1) kb ON true
  ORDER BY COALESCE(k.created_at,c.created_at) DESC`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var row adminGroupView
		if err := rows.Scan(&row.Couple, &row.KeyID, &row.Key, &row.Hint, &row.Label, &row.Created, &row.LastUsed,
			&row.MemberA, &row.GenderA, &row.RemarkA, &row.MemberB, &row.GenderB, &row.RemarkB,
			&row.DevicesA, &row.DevicesB, &row.Messages, &row.Revoked, &row.Archived,
			&row.KeyB, &row.HintB); err != nil {
			return nil, nil, err
		}
		row.HasKey = row.Key != ""
		row.HasKeyB = row.KeyB != ""
		row.GenderA = genderLabel(row.GenderA)
		row.GenderB = genderLabel(row.GenderB)
		row.BadgeClass = "off"
		switch {
		case row.Archived:
			row.Bound = "回收站"
		case row.DevicesA > 0 && row.DevicesB > 0:
			row.Bound, row.BadgeClass = "双方已登录", "ok"
		case row.DevicesA > 0 || row.DevicesB > 0:
			row.Bound, row.BadgeClass = "已登录 1 人", "warn"
		default:
			row.Bound = "未登录"
		}
		if row.Archived {
			binned = append(binned, row)
		} else {
			active = append(active, row)
		}
	}
	return active, binned, rows.Err()
}

func (s *Server) adminCreateKey(w http.ResponseWriter, r *http.Request) error {
	if _, ok := s.requireAdmin(w, r, false); !ok {
		return nil
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return nil
	}
	label := strings.TrimSpace(r.FormValue("label"))
	if len([]rune(label)) > 40 || !validOptionalText(label) {
		http.Redirect(w, r, "/admin?error=label", http.StatusSeeOther)
		return nil
	}
	key, _, err := s.createGroupAndKey(r.Context(), label)
	if err != nil {
		return err
	}
	http.Redirect(w, r, "/admin?created="+key, http.StatusSeeOther)
	return nil
}

// adminRevokeKey moves a whole group to the recycle bin: the key stops working
// immediately but the group and its chat history stay until it is purged.
func (s *Server) adminRevokeKey(w http.ResponseWriter, r *http.Request) error {
	if _, ok := s.requireAdmin(w, r, false); !ok {
		return nil
	}
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		return fail(400, "invalid_input")
	}
	var coupleID string
	err := s.db.QueryRow(r.Context(), `SELECT couple_id::text FROM access_keys WHERE id=$1`, id).Scan(&coupleID)
	if errors.Is(err, pgx.ErrNoRows) {
		redirectAdminError(w, r, "组不存在")
		return nil
	}
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `UPDATE access_keys SET revoked_at=now() WHERE couple_id=$1 AND revoked_at IS NULL`, coupleID); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `UPDATE couples SET archived_at=now() WHERE id=$1`, coupleID); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
	return nil
}

func (s *Server) adminRestoreKey(w http.ResponseWriter, r *http.Request) error {
	if _, ok := s.requireAdmin(w, r, false); !ok {
		return nil
	}
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		return fail(400, "invalid_input")
	}
	var coupleID string
	err := s.db.QueryRow(r.Context(), `SELECT couple_id::text FROM access_keys WHERE id=$1`, id).Scan(&coupleID)
	if errors.Is(err, pgx.ErrNoRows) {
		redirectAdminError(w, r, "组不存在")
		return nil
	}
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `UPDATE access_keys SET revoked_at=NULL WHERE couple_id=$1`, coupleID); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `UPDATE couples SET archived_at=NULL WHERE id=$1`, coupleID); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
	return nil
}

// adminPurgeKey permanently deletes one recycled group and all of its data.
func (s *Server) adminPurgeKey(w http.ResponseWriter, r *http.Request) error {
	if _, ok := s.requireAdmin(w, r, false); !ok {
		return nil
	}
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		return fail(400, "invalid_input")
	}
	var coupleID, userA, userB string
	err := s.db.QueryRow(r.Context(), `SELECT k.couple_id::text,c.user_a::text,c.user_b::text
  FROM access_keys k JOIN couples c ON c.id=k.couple_id WHERE k.id=$1`, id).Scan(&coupleID, &userA, &userB)
	if errors.Is(err, pgx.ErrNoRows) {
		redirectAdminError(w, r, "组不存在")
		return nil
	}
	if err != nil {
		return err
	}
	if err = s.purgeGroup(r.Context(), coupleID, userA, userB); err != nil {
		return err
	}
	http.Redirect(w, r, "/admin?changed=1", http.StatusSeeOther)
	return nil
}

// purgeGroup deletes a group's chat, memories, devices, sessions and accounts.
// Foreign keys require children to go first.
func (s *Server) purgeGroup(ctx context.Context, coupleID, userA, userB string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	steps := []struct {
		query string
		args  []any
	}{
		{`DELETE FROM message_receipts WHERE message_id IN (SELECT id FROM messages WHERE couple_id=$1)`, []any{coupleID}},
		{`DELETE FROM messages WHERE couple_id=$1`, []any{coupleID}},
		{`DELETE FROM interactions WHERE couple_id=$1`, []any{coupleID}},
		{`DELETE FROM anniversary_receipts WHERE reminder_id IN (SELECT r.id FROM anniversary_reminders r JOIN anniversaries a ON a.id=r.anniversary_id WHERE a.couple_id=$1)`, []any{coupleID}},
		{`DELETE FROM anniversary_reminders WHERE anniversary_id IN (SELECT id FROM anniversaries WHERE couple_id=$1)`, []any{coupleID}},
		{`DELETE FROM anniversaries WHERE couple_id=$1`, []any{coupleID}},
		{`DELETE FROM access_keys WHERE couple_id=$1`, []any{coupleID}},
		{`DELETE FROM couple_members WHERE couple_id=$1`, []any{coupleID}},
		{`DELETE FROM sync_events WHERE user_id IN ($1,$2)`, []any{userA, userB}},
		{`DELETE FROM sync_heads WHERE user_id IN ($1,$2)`, []any{userA, userB}},
		{`DELETE FROM sessions WHERE user_id IN ($1,$2)`, []any{userA, userB}},
		{`DELETE FROM devices WHERE user_id IN ($1,$2)`, []any{userA, userB}},
		{`DELETE FROM invitations WHERE creator_id IN ($1,$2)`, []any{userA, userB}},
		{`DELETE FROM couples WHERE id=$1`, []any{coupleID}},
		{`DELETE FROM users WHERE id IN ($1,$2)`, []any{userA, userB}},
	}
	for _, step := range steps {
		if _, err = tx.Exec(ctx, step.query, step.args...); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func renderAdmin(w http.ResponseWriter, body string, view any) error {
	tmpl, err := template.New("admin").Parse(adminHead + body + adminTail)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	return tmpl.Execute(w, view)
}

// adminIssueMemberKey upgrades a legacy shared-key group to separate A/B keys by
// claiming the existing key for member A and issuing a fresh key for member B.
func (s *Server) adminIssueMemberKey(w http.ResponseWriter, r *http.Request) error {
	if _, ok := s.requireAdmin(w, r, false); !ok {
		return nil
	}
	coupleID := r.PathValue("id")
	if !uuidPattern.MatchString(coupleID) {
		return fail(400, "invalid_input")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `UPDATE access_keys SET slot='a' WHERE couple_id=$1 AND slot IS NULL AND revoked_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM access_keys k2 WHERE k2.couple_id=$1 AND k2.slot='a')`, coupleID); err != nil {
		return err
	}
	var existing, label string
	err = tx.QueryRow(r.Context(), `SELECT id::text,COALESCE(label,'') FROM access_keys WHERE couple_id=$1 AND slot='b' LIMIT 1`, coupleID).Scan(&existing, &label)
	if err == nil {
		redirectAdminError(w, r, "该组已经有 B 密钥")
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_ = tx.QueryRow(r.Context(), `SELECT COALESCE(label,'') FROM access_keys WHERE couple_id=$1 AND slot='a' LIMIT 1`, coupleID).Scan(&label)
	key, err := newAccessKey()
	if err != nil {
		return err
	}
	keyID, err := identifier()
	if err != nil {
		return err
	}
	hint := key
	if len(hint) > 10 {
		hint = hint[:10]
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO access_keys(id,key_hash,key_plain,key_hint,couple_id,label,slot) VALUES($1,$2,$3,$4,$5,$6,'b')`, keyID, tokenHash(key), key, hint, coupleID, label); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	http.Redirect(w, r, "/admin?created="+key, http.StatusSeeOther)
	return nil
}