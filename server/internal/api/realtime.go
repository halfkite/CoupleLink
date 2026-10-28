package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func (s *Server) realtimeToken(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	if len(s.cfg.RealtimeSecret) < 32 || s.cfg.RealtimeURL == "" {
		return fail(503, "realtime_unavailable")
	}
	expires := time.Now().Add(time.Minute).Unix()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims, err := json.Marshal(map[string]any{"sub": id.SessionID, "channels": []string{"events:" + id.User.ID}, "exp": expires, "iat": time.Now().Unix(), "aud": "couplelink-realtime", "iss": "couplelink"})
	if err != nil {
		return err
	}
	unsigned := header + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, []byte(s.cfg.RealtimeSecret))
	mac.Write([]byte(unsigned))
	token := unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	write(w, 200, map[string]any{"token": token, "path": "/connection/websocket", "expires_at": expires})
	return nil
}
func (s *Server) attachRealtimeProxy(mux *http.ServeMux) {
	if s.cfg.RealtimeURL == "" {
		return
	}
	target, err := url.Parse(s.cfg.RealtimeURL)
	if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") {
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		write(w, 503, map[string]string{"code": "realtime_unavailable"})
	}
	mux.Handle("GET /connection/websocket", proxy)
}

// Only opaque refresh notifications go to WebSocket channels. Message bodies
// are read from the authenticated API, which rechecks the current relationship.
func (s *Server) RunRealtimeOutbox(ctx context.Context) {
	if s.cfg.RealtimeURL == "" || s.cfg.RealtimeAPIKey == "" {
		return
	}
	client := &http.Client{Timeout: 3 * time.Second}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			worked := s.publishOne(ctx, client)
			delay := time.Second
			if worked {
				delay = 10 * time.Millisecond
			}
			timer.Reset(delay)
		}
	}
}
func (s *Server) publishOne(ctx context.Context, client *http.Client) bool {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false
	}
	defer tx.Rollback(ctx)
	var id, kind, target string
	var attempts int
	err = tx.QueryRow(ctx, `SELECT id::text,kind,target,attempts FROM realtime_outbox WHERE next_attempt_at<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &kind, &target, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		return false
	}
	method := "publish"
	body := map[string]any{"channel": "events:" + target, "data": map[string]bool{"refresh": true}}
	if kind == "disconnect" {
		method = "disconnect"
		body = map[string]any{"user": target, "disconnect": map[string]any{"code": 3500, "reason": "session revoked", "reconnect": false}}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.RealtimeURL+"/api/"+method, bytes.NewReader(data))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.cfg.RealtimeAPIKey)
	response, callErr := client.Do(req)
	successful := false
	if callErr == nil {
		var result map[string]json.RawMessage
		decodeErr := json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&result)
		response.Body.Close()
		successful = response.StatusCode == 200 && decodeErr == nil && result["error"] == nil
	}
	if successful {
		_, err = tx.Exec(ctx, `DELETE FROM realtime_outbox WHERE id=$1`, id)
	} else {
		next := time.Now().Add(time.Duration(1<<min(attempts+1, 6)) * time.Second)
		_, err = tx.Exec(ctx, `UPDATE realtime_outbox SET attempts=attempts+1,next_attempt_at=$2 WHERE id=$1`, id, next)
	}
	if err != nil {
		return false
	}
	return tx.Commit(ctx) == nil
}
