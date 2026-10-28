package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPostgresAccountPairingAndRevocation(t *testing.T) {
	url := os.Getenv("COUPLELINK_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set COUPLELINK_TEST_DATABASE_URL for real PostgreSQL integration")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	suffix, err := identifier()
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_" + tokenHash(suffix)[:16]
	qualified := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+qualified); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+qualified+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal("migration must be repeatable", err)
	}
	service, err := New(db, Config{Version: "integration", AllowedOrigins: []string{"http://127.0.0.1:5178"}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()
	call := func(method, path, token string, body any) (int, map[string]any) {
		var buffer bytes.Buffer
		if body != nil {
			if err := json.NewEncoder(&buffer).Encode(body); err != nil {
				t.Error(err)
			}
		}
		req, err := http.NewRequest(method, server.URL+path, &buffer)
		if err != nil {
			t.Error(err)
			return 0, nil
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
			return 0, nil
		}
		defer res.Body.Close()
		var data map[string]any
		if err = json.NewDecoder(res.Body).Decode(&data); err != nil {
			t.Error(err)
		}
		return res.StatusCode, data
	}
	expect := func(want, status int, data map[string]any) {
		t.Helper()
		if status != want {
			t.Fatalf("HTTP status = %d, want %d; code=%v", status, want, data["code"])
		}
	}
	input := func(name string) map[string]any {
		device, err := identifier()
		if err != nil {
			t.Fatal(err)
		}
		return map[string]any{
			"username": name, "password": "integration password 123", "display_name": name,
			"device_id": device, "device_label": "integration phone", "platform": "android"}
	}
	a, b := input("alice"), input("bob")
	a["password"] = "12345678"
	for _, password := range []string{"", "1234567", strings.Repeat("a", maximumPasswordLength+1)} {
		invalid := input("invalid_password")
		invalid["password"] = password
		code, result := call("POST", "/auth/register", "", invalid)
		expect(400, code, result)
	}
	status, data := call("GET", "/account", "", nil)
	expect(401, status, data)
	status, data = call("POST", "/auth/register", "", a)
	expect(201, status, data)
	alice := data["token"].(string)
	status, data = call("POST", "/auth/register", "", b)
	expect(201, status, data)
	bob := data["token"].(string)
	status, data = call("POST", "/auth/register", "", input("third"))
	expect(201, status, data)
	third := data["token"].(string)
	status, data = call("POST", "/auth/register", "", input("third"))
	expect(409, status, data)
	status, data = call("POST", "/couples/invitations", alice, nil)
	expect(201, status, data)
	old := data["token"].(string)
	status, data = call("POST", "/couples/invitations", alice, nil)
	expect(201, status, data)
	invite := data["token"].(string)
	join := map[string]any{"token": old, "start_at": "2025-10-26", "timezone": "Asia/Hong_Kong"}
	status, data = call("POST", "/couples/join", bob, join)
	expect(400, status, data)
	join["token"] = invite
	status, data = call("POST", "/couples/join", alice, join)
	expect(400, status, data)
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for range 2 {
		wg.Go(func() { code, _ := call("POST", "/couples/join", bob, join); results <- code })
	}
	wg.Wait()
	close(results)
	successes := 0
	for code := range results {
		if code == 201 {
			successes++
		} else if code != 409 {
			t.Fatalf("concurrent join returned %d", code)
		}
	}
	if successes != 1 {
		t.Fatal("invite must bind exactly once")
	}
	status, data = call("GET", "/couples/current", alice, nil)
	expect(200, status, data)
	couple := data["couple"].(map[string]any)
	if couple["partner"].(map[string]any)["username"] != "bob" {
		t.Fatal("wrong partner")
	}
	status, data = call("GET", "/sync/changes?cursor=0", alice, nil)
	expect(200, status, data)
	if len(data["events"].([]any)) != 1 || data["next_cursor"] != float64(1) {
		t.Fatal("missing atomic bind event")
	}
	status, data = call("GET", "/couples/current", third, nil)
	expect(200, status, data)
	if data["couple"] != nil {
		t.Fatal("unrelated account can see another couple")
	}
	status, data = call("GET", "/sync/changes?cursor=0", third, nil)
	expect(200, status, data)
	if len(data["events"].([]any)) != 0 {
		t.Fatal("unrelated account received couple events")
	}
	interactionID, _ := identifier()
	interactionInput := map[string]any{"id": interactionID, "couple_id": couple["id"], "action": "poke"}
	status, data = call("POST", "/interactions", alice, interactionInput)
	expect(201, status, data)
	status, data = call("POST", "/interactions", alice, interactionInput)
	expect(200, status, data)
	newID, _ := identifier()
	status, data = call("POST", "/interactions", alice, map[string]any{"id": newID, "couple_id": couple["id"], "action": "vibrate"})
	expect(429, status, data)
	status, data = call("POST", "/interactions/claim", third, map[string]any{"couple_id": couple["id"]})
	expect(403, status, data)
	status, data = call("POST", "/interactions/claim", bob, map[string]any{"couple_id": couple["id"]})
	expect(200, status, data)
	if data["interaction"].(map[string]any)["id"] != interactionID || data["remaining_ms"].(float64) <= 0 {
		t.Fatal("wrong or expired interaction claimed")
	}
	status, data = call("POST", "/interactions/claim", bob, map[string]any{"couple_id": couple["id"]})
	expect(200, status, data)
	if data["interaction"] != nil {
		t.Fatal("same interaction claimed twice")
	}
	status, data = call("POST", "/interactions/result", bob, map[string]any{"id": interactionID, "couple_id": couple["id"], "result": "suppressed"})
	expect(200, status, data)
	status, data = call("GET", "/interactions?couple_id="+couple["id"].(string), alice, nil)
	expect(200, status, data)
	if data["interactions"].([]any)[0].(map[string]any)["result"] != "suppressed" {
		t.Fatal("feedback result was not recorded")
	}
	if _, err = db.Exec(ctx, `UPDATE interactions SET created_at=now()-interval '3 minutes' WHERE id=$1`, interactionID); err != nil {
		t.Fatal(err)
	}
	expiredID, _ := identifier()
	status, data = call("POST", "/interactions", alice, map[string]any{"id": expiredID, "couple_id": couple["id"], "action": "ring"})
	expect(201, status, data)
	if _, err = db.Exec(ctx, `UPDATE interactions SET expires_at=now()-interval '1 second' WHERE id=$1`, expiredID); err != nil {
		t.Fatal(err)
	}
	status, data = call("POST", "/interactions/claim", bob, map[string]any{"couple_id": couple["id"]})
	expect(200, status, data)
	if data["interaction"] != nil {
		t.Fatal("expired interaction must not be played")
	}
	if _, err = db.Exec(ctx, `UPDATE interactions SET created_at=now()-interval '30 seconds' WHERE sender_id=(SELECT id FROM users WHERE username='alice')`); err != nil {
		t.Fatal(err)
	}
	bobDevice := input("bob")
	delete(bobDevice, "display_name")
	status, data = call("POST", "/auth/login", "", bobDevice)
	expect(200, status, data)
	bobSecond := data["token"].(string)
	concurrentID, _ := identifier()
	status, data = call("POST", "/interactions", alice, map[string]any{"id": concurrentID, "couple_id": couple["id"], "action": "vibrate"})
	expect(201, status, data)
	claims := make(chan map[string]any, 2)
	var claimGroup sync.WaitGroup
	for _, token := range []string{bob, bobSecond} {
		claimGroup.Go(func() {
			status, result := call("POST", "/interactions/claim", token, map[string]any{"couple_id": couple["id"]})
			if status != 200 {
				t.Errorf("concurrent claim status = %d", status)
			}
			claims <- result
		})
	}
	claimGroup.Wait()
	close(claims)
	claimed := 0
	for result := range claims {
		if result["interaction"] != nil {
			claimed++
		}
	}
	if claimed != 1 {
		t.Fatalf("expected exactly one device claim, got %d", claimed)
	}
	messageID, _ := identifier()
	messageInput := map[string]any{"id": messageID, "couple_id": couple["id"], "body": "真实消息，断线重试使用相同 ID"}
	status, data = call("POST", "/messages", alice, messageInput)
	expect(201, status, data)
	if data["message"].(map[string]any)["seq"] != float64(1) {
		t.Fatal("first message sequence")
	}
	status, data = call("POST", "/messages", alice, messageInput)
	expect(200, status, data)
	var duplicateRequests sync.WaitGroup
	for range 8 {
		duplicateRequests.Go(func() {
			status, data := call("POST", "/messages", alice, messageInput)
			if status != 200 {
				t.Errorf("concurrent retry: status=%d code=%v", status, data["code"])
			}
		})
	}
	duplicateRequests.Wait()
	messageInput["body"] = "cannot replace accepted content"
	status, data = call("POST", "/messages", alice, messageInput)
	expect(409, status, data)
	messageInput["body"] = ""
	status, data = call("POST", "/messages", alice, messageInput)
	expect(400, status, data)
	messageInput["body"] = "third account cannot send to this relationship"
	status, data = call("POST", "/messages", third, messageInput)
	expect(403, status, data)
	status, data = call("GET", "/messages?couple_id="+couple["id"].(string), third, nil)
	expect(403, status, data)
	status, data = call("GET", "/messages?couple_id="+couple["id"].(string), bob, nil)
	expect(200, status, data)
	if len(data["messages"].([]any)) != 1 || data["messages"].([]any)[0].(map[string]any)["delivered"] != false {
		t.Fatal("server acceptance is not peer delivery")
	}
	status, data = call("POST", "/messages/receipts", bob, map[string]any{"couple_id": couple["id"], "through": 1})
	expect(200, status, data)
	status, data = call("GET", "/messages?couple_id="+couple["id"].(string), alice, nil)
	expect(200, status, data)
	if data["messages"].([]any)[0].(map[string]any)["delivered"] != true || data["peer_delivered_through"] != float64(1) {
		t.Fatal("receipt was not committed")
	}
	for index := 0; index < 100; index++ {
		id, _ := identifier()
		body := "pagination fixture"
		if index == 0 {
			body = strings.Repeat("心", 4000)
		}
		status, data = call("POST", "/messages", alice, map[string]any{"id": id, "couple_id": couple["id"], "body": body})
		expect(201, status, data)
	}
	status, data = call("GET", "/messages?couple_id="+couple["id"].(string), bob, nil)
	expect(200, status, data)
	if len(data["messages"].([]any)) != 100 || data["next_cursor"] != float64(100) || data["has_more"] != true {
		t.Fatal("first page boundary")
	}
	status, data = call("GET", "/messages?couple_id="+couple["id"].(string)+"&after=100", bob, nil)
	expect(200, status, data)
	if len(data["messages"].([]any)) != 1 || data["next_cursor"] != float64(101) {
		t.Fatal("incremental page lost or duplicated messages")
	}
	status, data = call("GET", "/messages?couple_id="+couple["id"].(string)+"&before=101", bob, nil)
	expect(200, status, data)
	if len(data["messages"].([]any)) != 100 || data["messages"].([]any)[0].(map[string]any)["seq"] != float64(1) {
		t.Fatal("history order")
	}
	testMemories(t, ctx, service, db, call, alice, bob, third, couple)
	var beforeUnbind int64
	if err = db.QueryRow(ctx, `SELECT seq FROM sync_heads WHERE user_id=(SELECT id FROM users WHERE username='bob')`).Scan(&beforeUnbind); err != nil {
		t.Fatal(err)
	}
	status, data = call("DELETE", "/couples/current", alice, map[string]any{"version": 999})
	expect(409, status, data)
	status, data = call("DELETE", "/couples/current", alice, map[string]any{"version": couple["version"]})
	expect(200, status, data)
	status, data = call("GET", "/couples/current", bob, nil)
	expect(200, status, data)
	if data["couple"] != nil {
		t.Fatal("partner retained revoked relationship")
	}
	for _, path := range []string{"/anniversaries?couple_id=", "/anniversaries/reminders?couple_id=", "/messages/search?q=fixture&couple_id="} {
		status, data = call("GET", path+couple["id"].(string), bob, nil)
		expect(403, status, data)
	}
	status, data = call("POST", "/couples/join", bob, join)
	expect(400, status, data)
	status, data = call("GET", fmt.Sprintf("/sync/changes?cursor=%d", beforeUnbind), bob, nil)
	expect(200, status, data)
	if data["events"].([]any)[0].(map[string]any)["type"] != "couple.unbound" {
		t.Fatal("missing unbind event")
	}
	status, data = call("GET", "/messages?couple_id="+couple["id"].(string), bob, nil)
	expect(403, status, data)
	messageInput["body"] = "old relationship must not accept delayed retries"
	status, data = call("POST", "/messages", alice, messageInput)
	expect(403, status, data)
	status, data = call("POST", "/interactions", alice, interactionInput)
	expect(403, status, data)
	status, data = call("GET", "/sync/changes?cursor=-1", bob, nil)
	expect(400, status, data)
	status, data = call("GET", "/sync/changes?cursor=999", bob, nil)
	expect(400, status, data)
	// A second device must have an independent session. Revoking it must neither
	// revoke another user's device nor leave its existing bearer token usable.
	second := input("alice")
	delete(second, "setup_code")
	delete(second, "display_name")
	second["password"] = a["password"]
	status, data = call("POST", "/auth/login", "", second)
	expect(200, status, data)
	secondToken := data["token"].(string)
	status, data = call("DELETE", "/devices/"+second["device_id"].(string), bob, nil)
	expect(404, status, data)
	status, data = call("DELETE", "/devices/"+second["device_id"].(string), alice, nil)
	expect(200, status, data)
	status, data = call("GET", "/account", secondToken, nil)
	expect(401, status, data)
	status, data = call("GET", "/account", alice, nil)
	expect(200, status, data)
	// Expiry is checked against database time, not a trusted client timestamp.
	status, data = call("POST", "/couples/invitations", bob, nil)
	expect(201, status, data)
	expired := data["token"].(string)
	if _, err = db.Exec(ctx, `UPDATE invitations SET expires_at=$1 WHERE token_hash=$2`, time.Now().Add(-time.Hour), tokenHash(expired)); err != nil {
		t.Fatal(err)
	}
	join["token"] = expired
	status, data = call("POST", "/couples/join", alice, join)
	expect(400, status, data)
	status, data = call("POST", "/auth/logout", alice, nil)
	expect(200, status, data)
	status, data = call("GET", "/account", alice, nil)
	expect(401, status, data)
	var stored string
	if err = db.QueryRow(ctx, `SELECT password_hash FROM users WHERE username='alice'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == a["password"] {
		t.Fatal("plaintext password stored")
	}
	var tokens int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE token_hash=$1`, alice).Scan(&tokens); err != nil {
		t.Fatal(err)
	}
	if tokens != 0 {
		t.Fatal("plaintext token stored")
	}
	// This schema belongs only to this test. Exercise outbox failures without
	// touching deployment queues or depending on an external broker.
	if _, err = db.Exec(ctx, `DELETE FROM realtime_outbox`); err != nil {
		t.Fatal(err)
	}
	brokerRejects := true
	broker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.URL.Path != "/api/publish" || r.Header.Get("X-API-Key") != strings.Repeat("a", 64) {
			t.Error("invalid broker request")
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		payload, _ := body["data"].(map[string]any)
		if len(payload) != 1 || payload["refresh"] != true {
			t.Error("WebSocket notification exposed extra data")
		}
		if brokerRejects {
			write(w, 200, map[string]any{"error": map[string]any{"code": 100}})
		} else {
			write(w, 200, map[string]any{"result": map[string]any{}})
		}
	}))
	defer broker.Close()
	service.cfg.RealtimeURL = broker.URL
	service.cfg.RealtimeAPIKey = strings.Repeat("a", 64)
	outboxID, err := identifier()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO realtime_outbox(id,kind,target) VALUES($1,'notify','test-recipient')`, outboxID); err != nil {
		t.Fatal(err)
	}
	service.publishOne(ctx, broker.Client())
	var attempts int
	if err = db.QueryRow(ctx, `SELECT attempts FROM realtime_outbox`).Scan(&attempts); err != nil || attempts != 1 {
		t.Fatal("rejected broker delivery must remain retryable", err, attempts)
	}
	brokerRejects = false
	if _, err = db.Exec(ctx, `UPDATE realtime_outbox SET next_attempt_at=now()`); err != nil {
		t.Fatal(err)
	}
	service.publishOne(ctx, broker.Client())
	var remaining int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM realtime_outbox`).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatal("successful broker retry was not acknowledged", err, remaining)
	}
	t.Log(fmt.Sprintf("real PostgreSQL verified: migration, no account cap or setup code, duplicate registration, concurrent single-use pairing, revisions, scoped sync, multi-device revocation"))
}
