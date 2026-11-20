package api

import (
	"context"
	"log"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"couplelink/server/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Version        string
	AllowedOrigins []string
	RealtimeURL    string
	RealtimeAPIKey string
	RealtimeSecret string
	AdminFile      string
	DownloadDir    string
	DataDir        string
	PublicURL      string
}
type Server struct {
	db        *pgxpool.Pool
	cfg       Config
	dummyHash string
	limiter   authLimiter
	hashSlots chan struct{}
}
type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Gender      string `json:"gender,omitempty"`
}
type Couple struct {
	ID            string `json:"id"`
	Partner       User   `json:"partner"`
	StartAt       string `json:"start_at"`
	Timezone      string `json:"timezone"`
	Version       int64  `json:"version"`
	Days          int    `json:"days_together"`
	MyRemark      string `json:"my_remark,omitempty"`
	PartnerRemark string `json:"partner_remark,omitempty"`
	MyAvatar      string `json:"my_avatar,omitempty"`
	PartnerAvatar string `json:"partner_avatar,omitempty"`
}
type identity struct {
	User                User
	SessionID, DeviceID string
}
type problem struct {
	status int
	code   string
}

func (p *problem) Error() string         { return p.code }
func fail(status int, code string) error { return &problem{status, code} }

var usernamePattern = regexp.MustCompile(`^[a-z0-9_-]{3,32}$`)

const minimumPasswordLength = 8
const maximumPasswordLength = 128

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func Migrate(ctx context.Context, db *pgxpool.Pool) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('couplelink.schema'))`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version integer PRIMARY KEY)`); err != nil {
		return err
	}
	var current int
	if err = tx.QueryRow(ctx, `SELECT COALESCE(max(version),0) FROM schema_migrations`).Scan(&current); err != nil {
		return err
	}
	if current > 16 {
		return errors.New("unsupported schema version")
	}
	if current == 0 {
		if _, err = tx.Exec(ctx, migrations.Accounts); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(1)`); err != nil {
			return err
		}
	}
	if current < 2 {
		if _, err = tx.Exec(ctx, migrations.Chat); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(2)`); err != nil {
			return err
		}
	}
	if current < 3 {
		if _, err = tx.Exec(ctx, migrations.Interactions); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(3)`); err != nil {
			return err
		}
	}
	if current < 4 {
		if _, err = tx.Exec(ctx, migrations.Memories); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(4)`); err != nil {
			return err
		}
	}
	if current < 5 {
		if _, err = tx.Exec(ctx, migrations.AccessKeys); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(5)`); err != nil {
			return err
		}
	}
	if current < 6 {
		if _, err = tx.Exec(ctx, migrations.Admin); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(6)`); err != nil {
			return err
		}
	}
	if current < 7 {
		if _, err = tx.Exec(ctx, migrations.AdminPlain); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(7)`); err != nil {
			return err
		}
	}
	if current < 8 {
		if _, err = tx.Exec(ctx, migrations.GroupArchive); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(8)`); err != nil {
			return err
		}
	}
	if current < 9 {
		if _, err = tx.Exec(ctx, migrations.ReadCursors); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(9)`); err != nil {
			return err
		}
	}
	if current < 10 {
		if _, err = tx.Exec(ctx, migrations.MessageRecall); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(10)`); err != nil {
			return err
		}
	}
	if current < 11 {
		if _, err = tx.Exec(ctx, migrations.MediaNotes); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(11)`); err != nil {
			return err
		}
	}
	if current < 12 {
		if _, err = tx.Exec(ctx, migrations.MessagePhotoBody); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(12)`); err != nil {
			return err
		}
	}
	if current < 13 {
		if _, err = tx.Exec(ctx, migrations.KeySlots); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(13)`); err != nil {
			return err
		}
	}
	if current < 14 {
		if _, err = tx.Exec(ctx, migrations.UserAvatar); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(14)`); err != nil {
			return err
		}
	}
	if current < 15 {
		if _, err = tx.Exec(ctx, migrations.NoteFeatured); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(15)`); err != nil {
			return err
		}
	}
	if current < 16 {
		if _, err = tx.Exec(ctx, migrations.CycleCare); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations VALUES(16)`); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func New(db *pgxpool.Pool, cfg Config) (*Server, error) {
	if cfg.RealtimeURL != "" || cfg.RealtimeAPIKey != "" || cfg.RealtimeSecret != "" {
		endpoint, err := url.Parse(cfg.RealtimeURL)
		if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.Path != "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || len(cfg.RealtimeAPIKey) < 32 || len(cfg.RealtimeSecret) < 32 {
			return nil, errors.New("invalid realtime configuration")
		}
	}
	dummy, err := hashPassword("constant dummy password for timing")
	if err != nil {
		return nil, err
	}
	return &Server{db: db, cfg: cfg, dummyHash: dummy, limiter: authLimiter{entries: map[string]rateEntry{}}, hashSlots: make(chan struct{}, 2)}, nil
}

type endpoint func(http.ResponseWriter, *http.Request) error

func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func decode(w http.ResponseWriter, r *http.Request, value any) error {
	return decodeLimit(w, r, value, 8192)
}
func decodeLimit(w http.ResponseWriter, r *http.Request, value any, limit int64) error {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return fail(415, "json_required")
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return fail(400, "invalid_input")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fail(400, "invalid_input")
	}
	return nil
}
func (s *Server) wrap(fn endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		if err := fn(w, r); err != nil {
			var p *problem
			var pe *pgconn.PgError
			switch {
			case errors.As(err, &p):
				write(w, p.status, map[string]string{"code": p.code})
			case errors.As(err, &pe) && pe.Code == "23505":
				write(w, 409, map[string]string{"code": "conflict"})
			default:
				log.Printf("internal error on %s %s: %v", r.Method, r.URL.Path, err)
				write(w, 500, map[string]string{"code": "server_error"})
			}
		}
	}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.wrap(s.health))
	mux.HandleFunc("POST /auth/key", s.wrap(s.keyLogin))
	mux.HandleFunc("GET /admin", s.wrap(s.adminPage))
	mux.HandleFunc("GET /admin/login", s.wrap(s.adminLoginPage))
	mux.HandleFunc("POST /admin/login", s.wrap(s.adminLoginSubmit))
	mux.HandleFunc("POST /admin/logout", s.wrap(s.adminLogout))
	mux.HandleFunc("GET /admin/password", s.wrap(s.adminPasswordPage))
	mux.HandleFunc("POST /admin/password", s.wrap(s.adminPasswordSubmit))
	mux.HandleFunc("POST /admin/keys", s.wrap(s.adminCreateKey))
	mux.HandleFunc("POST /admin/keys/{id}/revoke", s.wrap(s.adminRevokeKey))
	mux.HandleFunc("GET /admin/export", s.wrap(s.adminExport))
	mux.HandleFunc("POST /admin/import", s.wrap(s.adminImport))
	mux.HandleFunc("POST /admin/keys/{id}/restore", s.wrap(s.adminRestoreKey))
	mux.HandleFunc("POST /admin/keys/{id}/purge", s.wrap(s.adminPurgeKey))
	mux.HandleFunc("GET /admin/groups/{id}/export", s.wrap(s.adminGroupExport))
	mux.HandleFunc("POST /admin/groups/{id}/import", s.wrap(s.adminGroupImport))
	mux.HandleFunc("POST /admin/groups/{id}/issue-key", s.wrap(s.adminIssueMemberKey))
	mux.HandleFunc("POST /auth/register", s.wrap(s.register))
	mux.HandleFunc("POST /auth/login", s.wrap(s.login))
	mux.HandleFunc("POST /auth/logout", s.wrap(s.logout))
	mux.HandleFunc("GET /account", s.wrap(s.account))
	mux.HandleFunc("GET /couples/current", s.wrap(s.current))
	mux.HandleFunc("POST /couples/invitations", s.wrap(s.invite))
	mux.HandleFunc("POST /couples/join", s.wrap(s.join))
	mux.HandleFunc("DELETE /couples/current", s.wrap(s.unbind))
	mux.HandleFunc("GET /devices", s.wrap(s.devices))
	mux.HandleFunc("DELETE /devices/{id}", s.wrap(s.revokeDevice))
	mux.HandleFunc("GET /sync/changes", s.wrap(s.changes))
	mux.HandleFunc("POST /messages", s.wrap(s.sendMessage))
	mux.HandleFunc("GET /messages", s.wrap(s.listMessages))
	mux.HandleFunc("GET /messages/search", s.wrap(s.searchMessages))
	mux.HandleFunc("GET /messages/context", s.wrap(s.messageContext))
	mux.HandleFunc("PUT /couples/current", s.wrap(s.updateCoupleDate))
	mux.HandleFunc("GET /anniversaries", s.wrap(s.listAnniversaries))
	mux.HandleFunc("POST /anniversaries", s.wrap(s.saveAnniversary))
	mux.HandleFunc("PUT /anniversaries/{id}", s.wrap(s.saveAnniversary))
	mux.HandleFunc("DELETE /anniversaries/{id}", s.wrap(s.deleteAnniversary))
	mux.HandleFunc("GET /anniversaries/reminders", s.wrap(s.pendingReminders))
	mux.HandleFunc("POST /anniversaries/reminders/receipts", s.wrap(s.ackReminder))
	mux.HandleFunc("POST /messages/receipts", s.wrap(s.deliverMessages))
	mux.HandleFunc("POST /messages/read", s.wrap(s.markRead))
	mux.HandleFunc("POST /messages/{id}/recall", s.wrap(s.recallMessage))
	mux.HandleFunc("GET /update", s.wrap(s.updateManifest))
	mux.HandleFunc("GET /download/{name}", s.wrap(s.serveDownload))
	mux.HandleFunc("POST /images", s.wrap(s.uploadImage))
	mux.HandleFunc("GET /images/{id}", s.wrap(s.serveImage))
	mux.HandleFunc("GET /notes", s.wrap(s.listNotes))
	mux.HandleFunc("POST /notes", s.wrap(s.createNote))
	mux.HandleFunc("PUT /notes/{id}", s.wrap(s.updateNote))
	mux.HandleFunc("DELETE /notes/{id}", s.wrap(s.deleteNote))
	mux.HandleFunc("POST /notes/{id}/featured", s.wrap(s.setNoteFeatured))
	mux.HandleFunc("POST /interactions", s.wrap(s.sendInteraction))
	mux.HandleFunc("GET /interactions", s.wrap(s.listInteractions))
	mux.HandleFunc("POST /interactions/claim", s.wrap(s.claimInteraction))
	mux.HandleFunc("POST /interactions/result", s.wrap(s.interactionResult))
	mux.HandleFunc("POST /realtime/token", s.wrap(s.realtimeToken))
	mux.HandleFunc("PUT /me/remark", s.wrap(s.updateRemark))
	mux.HandleFunc("PUT /me/profile", s.wrap(s.updateProfile))
	mux.HandleFunc("PUT /me/avatar", s.wrap(s.updateAvatar))
	mux.HandleFunc("GET /care/profile", s.wrap(s.careGetProfile))
	mux.HandleFunc("PUT /care/profile", s.wrap(s.carePutProfile))
	mux.HandleFunc("GET /care/periods", s.wrap(s.careListPeriods))
	mux.HandleFunc("POST /care/periods", s.wrap(s.careSavePeriod))
	mux.HandleFunc("PUT /care/periods/{id}", s.wrap(s.careSavePeriod))
	mux.HandleFunc("DELETE /care/periods/{id}", s.wrap(s.careDeletePeriod))
	mux.HandleFunc("GET /care/logs", s.wrap(s.careListLogs))
	mux.HandleFunc("POST /care/logs", s.wrap(s.careSaveLog))
	mux.HandleFunc("PUT /care/logs/{id}", s.wrap(s.careSaveLog))
	mux.HandleFunc("DELETE /care/logs/{id}", s.wrap(s.careDeleteLog))
	mux.HandleFunc("GET /care/statistics", s.wrap(s.careStatistics))
	mux.HandleFunc("GET /care/preferences", s.wrap(s.careGetPreferences))
	mux.HandleFunc("PUT /care/preferences", s.wrap(s.carePutPreferences))
	mux.HandleFunc("GET /care/shares", s.wrap(s.careListShares))
	mux.HandleFunc("POST /care/shares", s.wrap(s.careGrantShare))
	mux.HandleFunc("DELETE /care/shares/{id}", s.wrap(s.careRevokeShare))
	mux.HandleFunc("GET /care/partner", s.wrap(s.carePartner))
	mux.HandleFunc("GET /care/tasks", s.wrap(s.careListTasks))
	mux.HandleFunc("POST /care/tasks", s.wrap(s.careSaveTask))
	mux.HandleFunc("PUT /care/tasks/{id}", s.wrap(s.careSaveTask))
	mux.HandleFunc("DELETE /care/tasks/{id}", s.wrap(s.careDeleteTask))
	mux.HandleFunc("GET /care/reminders", s.wrap(s.careListReminders))
	mux.HandleFunc("POST /care/reminders/receipts", s.wrap(s.careAckReminder))
	mux.HandleFunc("GET /care/dashboard", s.wrap(s.careDashboard))
	s.attachRealtimeProxy(mux)
	mux.HandleFunc("GET /", s.webApp)
	// Log page loads and sign-ins from other devices so a phone that cannot reach
	// the server can be told apart from one the app itself rejects.
	polled := map[string]bool{"/healthz": true, "/messages": true, "/sync/changes": true,
		"/interactions/claim": true, "/anniversaries/reminders": true, "/realtime/token": true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		media := strings.HasPrefix(r.URL.Path, "/notes") || strings.HasPrefix(r.URL.Path, "/images") || strings.HasPrefix(r.URL.Path, "/couples") || strings.HasPrefix(r.URL.Path, "/me")
		if media {
			sw := &statusWriter{ResponseWriter: w}
			w = sw
			defer func() { log.Printf("media request %s %s -> %d", r.Method, r.URL.Path, sw.status) }()
		}
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && host != "127.0.0.1" && host != "::1" && !polled[r.URL.Path] {
			log.Printf("访问 from=%s %s %s", host, r.Method, r.URL.Path)
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-CoupleLink-API", "1")
		if origin := r.Header.Get("Origin"); origin != "" && !strings.HasPrefix(r.URL.Path, "/admin") {
			allowed := origin == "http://"+r.Host || origin == "https://"+r.Host
			for _, value := range s.cfg.AllowedOrigins {
				if value == origin {
					allowed = true
				}
			}
			if !allowed {
				write(w, 403, map[string]string{"code": "origin_not_allowed"})
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(204)
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) error {
	if err := s.db.Ping(r.Context()); err != nil {
		return fail(503, "database_unavailable")
	}
	write(w, 200, map[string]any{"status": "ok", "api_version": 1, "version": s.cfg.Version})
	return nil
}
func (s *Server) auth(r *http.Request) (identity, error) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if len(token) != 43 || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		return identity{}, fail(401, "unauthorized")
	}
	var id identity
	err := s.db.QueryRow(r.Context(), `SELECT u.id::text,u.username,u.display_name,u.gender,s.id::text,d.id::text
  FROM sessions s JOIN users u ON u.id=s.user_id JOIN devices d ON d.id=s.device_id AND d.user_id=u.id
  WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND d.revoked_at IS NULL`, tokenHash(token)).Scan(
		&id.User.ID, &id.User.Username, &id.User.DisplayName, &id.User.Gender, &id.SessionID, &id.DeviceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, fail(401, "unauthorized")
	}
	return id, err
}

type authInput struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name,omitempty"`
	// Accepted only for request compatibility with r2 clients; no longer required.
	SetupCode   string `json:"setup_code,omitempty"`
	DeviceID    string `json:"device_id"`
	DeviceLabel string `json:"device_label"`
	Platform    string `json:"platform"`
}

func validText(text string, max int) bool {
	if !utf8.ValidString(text) || utf8.RuneCountInString(text) > max || strings.TrimSpace(text) == "" {
		return false
	}
	for _, r := range text {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
// validOptionalText accepts empty values and rejects control characters; it is
// used for free-form labels such as remarks that may legitimately be cleared.
func validOptionalText(text string) bool {
	if !utf8.ValidString(text) {
		return false
	}
	for _, r := range text {
		if unicode.IsControl(r) && r != '\t' {
			return false
		}
	}
	return true
}
func (s *Server) authRequest(w http.ResponseWriter, r *http.Request) (authInput, error) {
	var input authInput
	if err := decode(w, r, &input); err != nil {
		return input, err
	}
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !s.limiter.allow(host) {
		return input, fail(429, "rate_limited")
	}
	if !usernamePattern.MatchString(input.Username) || !uuidPattern.MatchString(input.DeviceID) || !validText(input.DeviceLabel, 64) || !validText(input.Platform, 24) || utf8.RuneCountInString(input.Password) < minimumPasswordLength || utf8.RuneCountInString(input.Password) > maximumPasswordLength || !utf8.ValidString(input.Password) {
		return input, fail(400, "invalid_input")
	}
	return input, nil
}
func (s *Server) reserveHash() bool {
	select {
	case s.hashSlots <- struct{}{}:
		return true
	default:
		return false
	}
}
func (s *Server) register(w http.ResponseWriter, r *http.Request) error {
	input, err := s.authRequest(w, r)
	if err != nil {
		return err
	}
	if !validText(input.DisplayName, 40) {
		return fail(400, "invalid_input")
	}
	if !s.reserveHash() {
		return fail(429, "rate_limited")
	}
	defer func() { <-s.hashSlots }()
	hash, err := hashPassword(input.Password)
	if err != nil {
		return err
	}
	id, err := identifier()
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `INSERT INTO users(id,username,display_name,password_hash) VALUES($1,$2,$3,$4)`, id, input.Username, input.DisplayName, hash); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO sync_heads(user_id) VALUES($1)`, id); err != nil {
		return err
	}
	token, expires, err := s.newSession(r.Context(), tx, id, input)
	if err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 201, map[string]any{"token": token, "expires_at": expires, "user": User{ID: id, Username: input.Username, DisplayName: input.DisplayName}})
	return nil
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	input, err := s.authRequest(w, r)
	if err != nil {
		return err
	}
	if !s.reserveHash() {
		return fail(429, "rate_limited")
	}
	defer func() { <-s.hashSlots }()
	var user User
	var hash string
	err = s.db.QueryRow(r.Context(), `SELECT id::text,username,display_name,password_hash FROM users WHERE username=$1`, input.Username).Scan(&user.ID, &user.Username, &user.DisplayName, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		verifyPassword(s.dummyHash, input.Password)
		return fail(401, "invalid_credentials")
	}
	if err != nil {
		return err
	}
	if !verifyPassword(hash, input.Password) {
		return fail(401, "invalid_credentials")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	token, expires, err := s.newSession(r.Context(), tx, user.ID, input)
	if err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]any{"token": token, "expires_at": expires, "user": user})
	return nil
}
func (s *Server) newSession(ctx context.Context, tx pgx.Tx, user string, input authInput) (string, time.Time, error) {
	if err := lockUsers(ctx, tx, user); err != nil {
		return "", time.Time{}, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO devices(id,user_id,label,platform) VALUES($1,$2,$3,$4)
  ON CONFLICT(id) DO UPDATE SET label=excluded.label,platform=excluded.platform,last_seen=now(),revoked_at=NULL
  WHERE devices.user_id=excluded.user_id`, input.DeviceID, user, input.DeviceLabel, input.Platform)
	if err != nil {
		return "", time.Time{}, err
	}
	if tag.RowsAffected() != 1 {
		return "", time.Time{}, fail(409, "device_in_use")
	}
	if err = revokeSessions(ctx, tx, user, input.DeviceID); err != nil {
		return "", time.Time{}, err
	}
	id, err := identifier()
	if err != nil {
		return "", time.Time{}, err
	}
	token, err := randomToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().UTC().Add(30 * 24 * time.Hour)
	_, err = tx.Exec(ctx, `INSERT INTO sessions(id,user_id,device_id,token_hash,expires_at) VALUES($1,$2,$3,$4,$5)`, id, user, input.DeviceID, tokenHash(token), expires)
	return token, expires, err
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if err = lockUsers(r.Context(), tx, id.User.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `UPDATE sessions SET revoked_at=now() WHERE id=$1`, id.SessionID); err != nil {
		return err
	}
	if err = enqueueRealtime(r.Context(), tx, "disconnect", id.SessionID); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]bool{"ok": true})
	return nil
}
func (s *Server) account(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	couple, err := s.readCouple(r.Context(), id.User.ID)
	if err != nil {
		return err
	}
	write(w, 200, map[string]any{"user": id.User, "couple": couple, "device_id": id.DeviceID, "features": []string{"memories_v1", "chat_search_v1", "cycle_care_v1"}})
	return nil
}
func (s *Server) current(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	couple, err := s.readCouple(r.Context(), id.User.ID)
	if err == nil {
		write(w, 200, map[string]any{"couple": couple})
	}
	return err
}
func (s *Server) readCouple(ctx context.Context, user string) (*Couple, error) {
	var c Couple
	var date time.Time
	err := s.db.QueryRow(ctx, `SELECT c.id::text,c.start_at,c.timezone,c.version,
  u.id::text,u.username,u.display_name,u.gender,
  CASE WHEN c.user_a=$1 THEN c.remark_a ELSE c.remark_b END,
  CASE WHEN c.user_a=$1 THEN c.remark_b ELSE c.remark_a END,
  COALESCE(u.avatar_image_id::text,''),(SELECT COALESCE(avatar_image_id::text,'') FROM users WHERE id=$1)
  FROM couple_members m JOIN couples c ON c.id=m.couple_id AND c.status='active'
  JOIN users u ON u.id=CASE WHEN c.user_a=$1 THEN c.user_b ELSE c.user_a END WHERE m.user_id=$1`, user).Scan(
		&c.ID, &date, &c.Timezone, &c.Version, &c.Partner.ID, &c.Partner.Username, &c.Partner.DisplayName, &c.Partner.Gender,
		&c.MyRemark, &c.PartnerRemark, &c.PartnerAvatar, &c.MyAvatar)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.StartAt = date.Format(time.DateOnly)
	zone, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return nil, err
	}
	today := time.Now().In(zone)
	localDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	c.Days = int(localDate.Sub(date.UTC()).Hours() / 24)
	return &c, nil
}
func lockUsers(ctx context.Context, tx pgx.Tx, ids ...string) error {
	sort.Strings(ids)
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, id); err != nil {
			return err
		}
	}
	return nil
}
func unbound(ctx context.Context, tx pgx.Tx, user string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM couple_members WHERE user_id=$1)`, user).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return fail(409, "already_bound")
	}
	return nil
}
func event(ctx context.Context, tx pgx.Tx, kind, ref string, ids ...string) error {
	sort.Strings(ids)
	for _, id := range ids {
		var seq int64
		if err := tx.QueryRow(ctx, `UPDATE sync_heads SET seq=seq+1 WHERE user_id=$1 RETURNING seq`, id).Scan(&seq); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO sync_events(user_id,seq,event_type,ref_id) VALUES($1,$2,$3,$4)`, id, seq, kind, ref); err != nil {
			return err
		}
		if err := enqueueRealtime(ctx, tx, "notify", id); err != nil {
			return err
		}
	}
	return nil
}
func (s *Server) invite(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if err = lockUsers(r.Context(), tx, id.User.ID); err != nil {
		return err
	}
	if err = unbound(r.Context(), tx, id.User.ID); err != nil {
		return err
	}
	token, err := randomToken()
	if err != nil {
		return err
	}
	inviteID, err := identifier()
	if err != nil {
		return err
	}
	expires := time.Now().UTC().Add(15 * time.Minute)
	if _, err = tx.Exec(r.Context(), `UPDATE invitations SET revoked_at=now() WHERE creator_id=$1 AND used_at IS NULL AND revoked_at IS NULL`, id.User.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO invitations(id,creator_id,token_hash,expires_at) VALUES($1,$2,$3,$4)`, inviteID, id.User.ID, tokenHash(token), expires); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 201, map[string]any{"token": token, "expires_at": expires})
	return nil
}
func (s *Server) join(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	var input struct {
		Token    string `json:"token"`
		StartAt  string `json:"start_at"`
		Timezone string `json:"timezone"`
	}
	if err = decode(w, r, &input); err != nil {
		return err
	}
	if len(input.Token) != 43 {
		return fail(400, "invalid_invitation")
	}
	zone, err := time.LoadLocation(input.Timezone)
	if err != nil || input.Timezone == "" {
		return fail(400, "invalid_input")
	}
	start, err := time.Parse(time.DateOnly, input.StartAt)
	if err != nil || input.StartAt > time.Now().In(zone).Format(time.DateOnly) {
		return fail(400, "invalid_input")
	}
	var creator string
	err = s.db.QueryRow(r.Context(), `SELECT creator_id::text FROM invitations WHERE token_hash=$1`, tokenHash(input.Token)).Scan(&creator)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(400, "invalid_invitation")
	}
	if err != nil {
		return err
	}
	if creator == id.User.ID {
		return fail(400, "self_invitation")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if err = lockUsers(r.Context(), tx, creator, id.User.ID); err != nil {
		return err
	}
	for _, user := range []string{creator, id.User.ID} {
		if err = unbound(r.Context(), tx, user); err != nil {
			return err
		}
	}
	var valid bool
	err = tx.QueryRow(r.Context(), `SELECT expires_at>now() AND used_at IS NULL AND revoked_at IS NULL FROM invitations WHERE token_hash=$1 FOR UPDATE`, tokenHash(input.Token)).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return fail(400, "invalid_invitation")
	}
	coupleID, err := identifier()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO couples(id,user_a,user_b,start_at,timezone,status) VALUES($1,$2,$3,$4,$5,'active')`, coupleID, creator, id.User.ID, start, input.Timezone); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO couple_members(user_id,couple_id) VALUES($1,$3),($2,$3)`, creator, id.User.ID, coupleID); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `UPDATE invitations SET used_at=now() WHERE token_hash=$1`, tokenHash(input.Token)); err != nil {
		return err
	}
	if err = event(r.Context(), tx, "couple.bound", coupleID, creator, id.User.ID); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	couple, err := s.readCouple(r.Context(), id.User.ID)
	if err == nil {
		write(w, 201, map[string]any{"couple": couple})
	}
	return err
}
func (s *Server) unbind(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	var input struct {
		Version int64 `json:"version"`
	}
	if err = decode(w, r, &input); err != nil {
		return err
	}
	var coupleID, a, b string
	err = s.db.QueryRow(r.Context(), `SELECT c.id::text,c.user_a::text,c.user_b::text FROM couple_members m JOIN couples c ON c.id=m.couple_id WHERE m.user_id=$1`, id.User.ID).Scan(&coupleID, &a, &b)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(409, "not_bound")
	}
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if err = lockUsers(r.Context(), tx, a, b); err != nil {
		return err
	}
	tag, err := tx.Exec(r.Context(), `UPDATE couples SET status='ended',version=version+1 WHERE id=$1 AND status='active' AND version=$2`, coupleID, input.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fail(409, "version_conflict")
	}
	if _, err = tx.Exec(r.Context(), `DELETE FROM couple_members WHERE couple_id=$1`, coupleID); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `UPDATE invitations SET revoked_at=now() WHERE creator_id IN ($1,$2) AND used_at IS NULL AND revoked_at IS NULL`, a, b); err != nil {
		return err
	}
	// Sharing is only meaningful inside an active relationship: revoke it in the
	// same transaction so a partner cannot read stale health data after unbind.
	if _, err = tx.Exec(r.Context(), `UPDATE care_share_permissions SET revoked_at=now() WHERE (owner_user_id=$1 OR owner_user_id=$2) AND revoked_at IS NULL`, a, b); err != nil {
		return err
	}
	if err = event(r.Context(), tx, "couple.unbound", coupleID, a, b); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]bool{"ok": true})
	return nil
}
func (s *Server) devices(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	rows, err := s.db.Query(r.Context(), `SELECT id::text,label,platform,last_seen FROM devices WHERE user_id=$1 AND revoked_at IS NULL ORDER BY last_seen DESC`, id.User.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	values := []map[string]any{}
	for rows.Next() {
		var deviceID, label, platform string
		var seen time.Time
		if err = rows.Scan(&deviceID, &label, &platform, &seen); err != nil {
			return err
		}
		values = append(values, map[string]any{"id": deviceID, "label": label, "platform": platform, "last_seen": seen, "current": deviceID == id.DeviceID})
	}
	if err = rows.Err(); err != nil {
		return err
	}
	write(w, 200, map[string]any{"devices": values})
	return nil
}
func (s *Server) revokeDevice(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	device := r.PathValue("id")
	if !uuidPattern.MatchString(device) {
		return fail(400, "invalid_input")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if err = lockUsers(r.Context(), tx, id.User.ID); err != nil {
		return err
	}
	tag, err := tx.Exec(r.Context(), `UPDATE devices SET revoked_at=now() WHERE id=$1 AND user_id=$2`, device, id.User.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fail(404, "device_not_found")
	}
	if err = revokeSessions(r.Context(), tx, id.User.ID, device); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]bool{"ok": true})
	return nil
}
func (s *Server) changes(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	cursor := int64(0)
	if value := r.URL.Query().Get("cursor"); value != "" {
		cursor, err = strconv.ParseInt(value, 10, 64)
		if err != nil || cursor < 0 {
			return fail(400, "invalid_cursor")
		}
	}
	var head int64
	if err = s.db.QueryRow(r.Context(), `SELECT seq FROM sync_heads WHERE user_id=$1`, id.User.ID).Scan(&head); err != nil {
		return err
	}
	if cursor > head {
		return fail(400, "invalid_cursor")
	}
	rows, err := s.db.Query(r.Context(), `SELECT seq,event_type,ref_id::text,created_at FROM sync_events WHERE user_id=$1 AND seq>$2 ORDER BY seq LIMIT 100`, id.User.ID, cursor)
	if err != nil {
		return err
	}
	defer rows.Close()
	events := []map[string]any{}
	next := cursor
	for rows.Next() {
		var seq int64
		var kind, ref string
		var created time.Time
		if err = rows.Scan(&seq, &kind, &ref, &created); err != nil {
			return err
		}
		events = append(events, map[string]any{"seq": seq, "type": kind, "ref_id": ref, "created_at": created})
		next = seq
	}
	if err = rows.Err(); err != nil {
		return err
	}
	write(w, 200, map[string]any{"events": events, "next_cursor": next, "has_more": len(events) == 100})
	return nil
}

// statusWriter records the status code for diagnostics.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}
