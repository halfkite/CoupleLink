package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
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
	// Simulate a 1.0-r63 database (schema version 15) to prove 016 upgrades in
	// place instead of only working on an empty schema.
	if _, err = db.Exec(ctx, `DROP TABLE IF EXISTS cycle_reminder_receipts,cycle_reminders,care_tasks,
  care_share_permissions,care_preferences,cycle_daily_logs,period_records,cycle_profiles CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `DELETE FROM schema_migrations WHERE version=16`); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal("016 must upgrade a version 15 schema", err)
	}
	var careMigration int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE version=16`).Scan(&careMigration); err != nil {
		t.Fatal(err)
	}
	if careMigration != 1 {
		t.Fatal("016 did not record its schema version")
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
	careImage := testCare(t, ctx, service, db, server.URL, call, alice, bob, third, couple)
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
	// Care data and the old relationship's notes and images must be unreachable
	// after unbind, including through a still valid session token.
	status, data = call("GET", "/care/partner", bob, nil)
	expect(403, status, data)
	status, data = call("GET", "/care/tasks", bob, nil)
	expect(403, status, data)
	status, data = call("GET", "/care/shares", alice, nil)
	expect(200, status, data)
	if len(data["shares"].([]any)) != 0 {
		t.Fatal("unbind must revoke every care share")
	}
	status, data = call("GET", "/notes?couple_id="+couple["id"].(string), bob, nil)
	expect(403, status, data)
	status, data = call("GET", "/images/"+careImage+"?token="+alice, "", nil)
	expect(403, status, data)
	var activeShares int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM care_share_permissions WHERE revoked_at IS NULL`).Scan(&activeShares); err != nil {
		t.Fatal(err)
	}
	if activeShares != 0 {
		t.Fatalf("unbind left %d active care shares", activeShares)
	}
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

// testCare exercises the cycle care contract against a real database and returns
// the uploaded image id so the caller can assert the post-unbind regression.
func testCare(t *testing.T, ctx context.Context, s *Server, db *pgxpool.Pool, baseURL string, call func(string, string, string, any) (int, map[string]any), alice, bob, third string, couple map[string]any) string {
	t.Helper()
	cid := couple["id"].(string)
	check := func(want int, method, path, token string, body any) map[string]any {
		t.Helper()
		status, data := call(method, path, token, body)
		if status != want {
			t.Fatalf("%s %s status %d, want %d; code %v", method, path, status, want, data["code"])
		}
		return data
	}
	rawStatus := func(url string) int {
		t.Helper()
		res, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if _, err = io.Copy(io.Discard, res.Body); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode
	}
	base := time.Now().In(chinaZone())
	day := func(offset int) string { return base.AddDate(0, 0, offset).Format(time.DateOnly) }
	// Image serving needs a data directory; the shared service starts without one
	// because only the unbind regression uploads a file.
	if s.cfg.DataDir == "" {
		s.cfg.DataDir = t.TempDir()
	}
	var aliceID string
	if err := db.QueryRow(ctx, `SELECT id::text FROM users WHERE username='alice'`).Scan(&aliceID); err != nil {
		t.Fatal(err)
	}

	// Authentication and per-user isolation.
	status, data := call("GET", "/care/profile", "", nil)
	if status != 401 {
		t.Fatalf("anonymous profile read = %d", status)
	}
	profile := check(200, "GET", "/care/profile", third, nil)["profile"].(map[string]any)
	if profile["timezone"] != "Asia/Shanghai" || profile["average_cycle_days"] != float64(28) || profile["reminder_enabled"] != true {
		t.Fatalf("default care profile = %v", profile)
	}
	if len(check(200, "GET", "/care/periods", third, nil)["periods"].([]any)) != 0 {
		t.Fatal("new account inherited period records")
	}
	check(403, "GET", "/care/partner", third, nil)
	check(403, "GET", "/care/tasks", third, nil)
	check(403, "POST", "/care/shares", third, map[string]any{"scope": "cycle_summary"})
	thirdPeriod := map[string]any{"start_date": day(-3), "end_date": day(-1)}
	check(201, "POST", "/care/periods", third, thirdPeriod)
	if len(check(200, "GET", "/care/periods", alice, nil)["periods"].([]any)) != 0 {
		t.Fatal("one account's period records leaked to another")
	}

	// Profile defaults, updates and bounds.
	aliceProfile := check(200, "GET", "/care/profile", alice, nil)["profile"].(map[string]any)
	if aliceProfile["average_cycle_days"] != float64(28) || aliceProfile["average_period_days"] != float64(5) {
		t.Fatalf("default profile = %v", aliceProfile)
	}
	check(200, "PUT", "/care/profile", alice, map[string]any{"average_cycle_days": 30})
	if check(200, "GET", "/care/profile", alice, nil)["profile"].(map[string]any)["average_cycle_days"] != float64(30) {
		t.Fatal("profile update was not stored")
	}
	check(400, "PUT", "/care/profile", alice, map[string]any{"average_cycle_days": 20})
	check(400, "PUT", "/care/profile", alice, map[string]any{"average_cycle_days": 46})
	check(400, "PUT", "/care/profile", alice, map[string]any{"average_period_days": 11})
	check(400, "PUT", "/care/profile", alice, map[string]any{"timezone": "Mars/Base"})
	if code := check(200, "GET", "/account", alice, nil); !careContains4(code) {
		t.Fatal("account must advertise cycle_care_v1")
	}

	// Period records: create, validate, statistics, update and delete.
	first := map[string]any{"start_date": day(-58), "end_date": day(-54), "flow_level": "medium", "note": ""}
	firstID := check(201, "POST", "/care/periods", alice, first)["period"].(map[string]any)["id"].(string)
	second := map[string]any{"start_date": day(-28), "end_date": day(-24)}
	secondID := check(201, "POST", "/care/periods", alice, second)["period"].(map[string]any)["id"].(string)
	if secondID == firstID {
		t.Fatal("period ids must be generated per record")
	}
	check(400, "POST", "/care/periods", alice, map[string]any{"start_date": "2026-02-30"})
	check(400, "POST", "/care/periods", alice, map[string]any{"start_date": day(1)})
	check(400, "POST", "/care/periods", alice, map[string]any{"start_date": day(-10), "end_date": day(-12)})
	check(400, "POST", "/care/periods", alice, map[string]any{"start_date": day(-10), "flow_level": "extreme"})
	check(400, "POST", "/care/periods", alice, map[string]any{"start_date": day(-10), "note": strings.Repeat("a", careMaxNoteRunes+1)})
	if code := check(409, "POST", "/care/periods", alice, map[string]any{"start_date": day(-28), "end_date": day(-24)}); code["code"] != "cycle_overlap" {
		t.Fatalf("duplicate start code = %v", code["code"])
	}
	if code := check(409, "POST", "/care/periods", alice, map[string]any{"start_date": day(-55), "end_date": day(-53)}); code["code"] != "cycle_overlap" {
		t.Fatalf("overlapping range code = %v", code["code"])
	}
	statistics := check(200, "GET", "/care/statistics", alice, nil)["statistics"].(map[string]any)
	if statistics["average_cycle_days"] != float64(30) || statistics["average_period_days"] != float64(5) {
		t.Fatalf("statistics averages = %v/%v", statistics["average_cycle_days"], statistics["average_period_days"])
	}
	if statistics["predicted_start_date"] != day(2) || statistics["predicted_end_date"] != day(6) || statistics["days_until_next"] != float64(2) {
		t.Fatalf("statistics prediction = %v/%v/%v", statistics["predicted_start_date"], statistics["predicted_end_date"], statistics["days_until_next"])
	}
	if statistics["confidence"] != "estimate" {
		t.Fatal("predictions must stay labelled as estimates")
	}
	cycles := statistics["cycles"].([]any)
	if len(cycles) != 2 || cycles[0].(map[string]any)["length_days"] != float64(30) || cycles[1].(map[string]any)["length_days"] != nil {
		t.Fatalf("cycle history = %v", cycles)
	}
	check(200, "PUT", "/care/periods/"+firstID, alice, map[string]any{"start_date": day(-58), "end_date": day(-54), "flow_level": "heavy"})
	if list := check(200, "GET", "/care/periods?from="+day(-70)+"&to="+day(0), alice, nil)["periods"].([]any); len(list) != 2 || list[0].(map[string]any)["flow_level"] != "heavy" {
		t.Fatalf("period update = %v", list)
	}
	throwaway := check(201, "POST", "/care/periods", alice, map[string]any{"start_date": day(-100), "end_date": day(-98)})["period"].(map[string]any)["id"].(string)
	check(200, "DELETE", "/care/periods/"+throwaway, alice, nil)
	check(404, "DELETE", "/care/periods/"+throwaway, alice, nil)
	if len(check(200, "GET", "/care/periods", alice, nil)["periods"].([]any)) != 2 {
		t.Fatal("period delete did not remove the record")
	}

	// Daily logs: enums, duplicate day and update.
	logInput := map[string]any{"log_date": day(0), "pain_level": 5, "mood": "tired", "symptoms": []string{"cramps", "headache"}, "note": "记录"}
	logRecord := check(201, "POST", "/care/logs", alice, logInput)["log"].(map[string]any)
	logID := logRecord["id"].(string)
	if len(logRecord["symptoms"].([]any)) != 2 {
		t.Fatal("symptoms were not stored")
	}
	if code := check(409, "POST", "/care/logs", alice, logInput); code["code"] != "conflict" {
		t.Fatalf("duplicate day code = %v", code["code"])
	}
	for _, invalid := range []map[string]any{
		{"log_date": day(0), "pain_level": 11},
		{"log_date": day(0), "pain_level": -1},
		{"log_date": day(0), "mood": "angry"},
		{"log_date": day(0), "symptoms": []string{"fever"}},
		{"log_date": "2026-02-30"},
		{"log_date": day(1)},
		{"log_date": day(0), "note": strings.Repeat("a", careMaxNoteRunes+1)},
	} {
		check(400, "POST", "/care/logs", alice, invalid)
	}
	check(200, "PUT", "/care/logs/"+logID, alice, map[string]any{"log_date": day(0), "pain_level": 3, "mood": "calm", "symptoms": []string{"cramps"}, "note": ""})
	logs := check(200, "GET", "/care/logs", alice, nil)["logs"].([]any)
	if len(logs) != 1 || logs[0].(map[string]any)["pain_level"] != float64(3) || len(logs[0].(map[string]any)["symptoms"].([]any)) != 1 {
		t.Fatalf("log update = %v", logs)
	}
	check(200, "DELETE", "/care/logs/"+logID, alice, nil)
	if len(check(200, "GET", "/care/logs", alice, nil)["logs"].([]any)) != 0 {
		t.Fatal("log delete did not remove the record")
	}
	check(201, "POST", "/care/logs", alice, map[string]any{"log_date": day(0), "pain_level": 2, "mood": "calm"})

	// Care preferences are personal and validated.
	preferences := check(200, "GET", "/care/preferences", alice, nil)["preferences"].(map[string]any)
	if len(preferences["preferred_support"].([]any)) != 0 {
		t.Fatalf("default preferences = %v", preferences)
	}
	check(200, "PUT", "/care/preferences", alice, map[string]any{"preferred_support": []string{"抱抱"}, "avoid_support": []string{"说教"}, "note": "别催"})
	preferences = check(200, "GET", "/care/preferences", alice, nil)["preferences"].(map[string]any)
	if preferences["note"] != "别催" || len(preferences["preferred_support"].([]any)) != 1 {
		t.Fatalf("preferences were not stored: %v", preferences)
	}
	check(400, "PUT", "/care/preferences", alice, map[string]any{"note": strings.Repeat("a", careMaxPreferenceNote+1)})

	// Sharing is opt-in per scope and revocation hides data immediately.
	page := check(200, "GET", "/care/shares", alice, nil)
	if len(page["shares"].([]any)) != 0 || len(page["scopes"].([]any)) != len(careScopeList) {
		t.Fatalf("initial shares page = %v", page)
	}
	share := check(201, "POST", "/care/shares", alice, map[string]any{"scope": "cycle_summary"})["share"].(map[string]any)
	shareID := share["id"].(string)
	if share["scope"] != "cycle_summary" {
		t.Fatal("share scope was not stored")
	}
	if again := check(200, "POST", "/care/shares", alice, map[string]any{"scope": "cycle_summary"})["share"].(map[string]any); again["id"] != shareID {
		t.Fatal("repeat grant must return the existing share")
	}
	check(400, "POST", "/care/shares", alice, map[string]any{"scope": "everything"})
	check(400, "POST", "/care/shares", alice, map[string]any{})
	partner := check(200, "GET", "/care/partner", bob, nil)["partner"].(map[string]any)
	grants := partner["grants"].(map[string]any)
	if grants["cycle_summary"] != true || grants["period_dates"] != false || grants["daily_logs"] != false {
		t.Fatalf("partner grants = %v", grants)
	}
	if partner["periods"] != nil || partner["logs"] != nil || partner["preferences"] != nil {
		t.Fatal("ungranted scopes must stay null")
	}
	if partner["summary"] == nil {
		t.Fatal("the granted cycle summary must be returned")
	}
	dates := check(201, "POST", "/care/shares", alice, map[string]any{"scope": "period_dates"})["share"].(map[string]any)
	datesID := dates["id"].(string)
	partner = check(200, "GET", "/care/partner", bob, nil)["partner"].(map[string]any)
	if len(partner["periods"].([]any)) != 2 {
		t.Fatalf("granted period dates = %v", partner["periods"])
	}
	check(200, "DELETE", "/care/shares/"+datesID, alice, nil)
	partner = check(200, "GET", "/care/partner", bob, nil)["partner"].(map[string]any)
	if partner["periods"] != nil {
		t.Fatal("revoked scope must be hidden immediately")
	}
	check(403, "DELETE", "/care/shares/"+shareID, bob, nil)
	missing, _ := identifier()
	check(404, "DELETE", "/care/shares/"+missing, alice, nil)
	check(403, "GET", "/care/partner", third, nil)

	// The care checklist belongs to the active couple.
	task := check(201, "POST", "/care/tasks", alice, map[string]any{"title": "提醒喝水", "note": "多喝热水"})["task"].(map[string]any)
	taskID := task["id"].(string)
	if task["status"] != "open" || task["note"] != "多喝热水" || task["created_by_me"] != true || task["subject_me"] != false {
		t.Fatalf("created task = %v", task)
	}
	bobTasks := check(200, "GET", "/care/tasks", bob, nil)["tasks"].([]any)
	if len(bobTasks) != 1 {
		t.Fatalf("partner cannot see the care tasks: %v", bobTasks)
	}
	if seen := bobTasks[0].(map[string]any); seen["subject_me"] != true || seen["created_by_me"] != false {
		t.Fatalf("task ownership flags = %v", seen)
	}
	check(200, "PUT", "/care/tasks/"+taskID, bob, map[string]any{"status": "done"})
	done := check(200, "GET", "/care/tasks?status=done", alice, nil)["tasks"].([]any)
	if len(done) != 1 || done[0].(map[string]any)["status"] != "done" || done[0].(map[string]any)["completed_at"] == nil {
		t.Fatalf("task completion = %v", done)
	}
	if len(check(200, "GET", "/care/tasks?status=open", alice, nil)["tasks"].([]any)) != 0 {
		t.Fatal("completed task still listed as open")
	}
	check(400, "GET", "/care/tasks?status=archived", alice, nil)
	check(400, "PUT", "/care/tasks/"+taskID, alice, map[string]any{"status": "archived"})
	check(403, "GET", "/care/tasks", third, nil)
	check(403, "POST", "/care/tasks", third, map[string]any{"title": "越权"})
	check(200, "DELETE", "/care/tasks/"+taskID, alice, nil)
	check(404, "DELETE", "/care/tasks/"+taskID, alice, nil)

	// Dashboard reports the same prediction and the sharing state.
	dashboard := check(200, "GET", "/care/dashboard", alice, nil)["dashboard"].(map[string]any)
	todayState := dashboard["today"].(map[string]any)
	if todayState["in_period"] != false || todayState["has_log"] != true || todayState["open_tasks"] != float64(0) {
		t.Fatalf("dashboard today = %v", todayState)
	}
	prediction := dashboard["prediction"].(map[string]any)
	if prediction["start_date"] != day(2) || prediction["days_until"] != float64(2) || prediction["estimate"] != true {
		t.Fatalf("dashboard prediction = %v", prediction)
	}
	if dashboard["partner"].(map[string]any)["summary"] != nil {
		t.Fatal("dashboard must not invent a partner summary without a grant")
	}
	dashboardShares := dashboard["shares"].(map[string]any)
	if len(dashboardShares["active"].([]any)) != 1 || dashboardShares["active"].([]any)[0] != "cycle_summary" || len(dashboardShares["available"].([]any)) != 3 {
		t.Fatalf("dashboard shares = %v", dashboardShares)
	}
	bobDashboard := check(200, "GET", "/care/dashboard", bob, nil)["dashboard"].(map[string]any)
	if bobDashboard["partner"].(map[string]any)["summary"] == nil {
		t.Fatal("the granted partner summary must appear on the partner dashboard")
	}
	thirdDashboard := check(200, "GET", "/care/dashboard", third, nil)["dashboard"].(map[string]any)
	if thirdDashboard["partner"] != nil || len(thirdDashboard["shares"].(map[string]any)["active"].([]any)) != 0 {
		t.Fatalf("unbound dashboard = %v", thirdDashboard)
	}

	// Reminder generation is idempotent and receipts are per device.
	for range 2 {
		if err := s.generateCareReminders(ctx, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	var generated int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM cycle_reminders WHERE user_id=$1 AND kind='period_upcoming'`, aliceID).Scan(&generated); err != nil {
		t.Fatal(err)
	}
	if generated != 1 {
		t.Fatalf("reminder dedup produced %d rows", generated)
	}
	if _, err := db.Exec(ctx, `UPDATE cycle_reminders SET scheduled_at=now()-interval '5 minutes', expires_at=now()+interval '24 hours' WHERE user_id=$1 AND kind='period_upcoming'`, aliceID); err != nil {
		t.Fatal(err)
	}
	secondDevice, _ := identifier()
	status, data = call("POST", "/auth/login", "", map[string]any{"username": "alice", "password": "12345678", "device_id": secondDevice, "device_label": "second phone", "platform": "android"})
	if status != 200 {
		t.Fatalf("second alice login status %d code %v", status, data["code"])
	}
	aliceSecond := data["token"].(string)
	if status, _ = call("GET", "/care/reminders", "", nil); status != 401 {
		t.Fatalf("anonymous reminders = %d", status)
	}
	due := check(200, "GET", "/care/reminders", alice, nil)["reminders"].([]any)
	if len(due) != 1 {
		t.Fatalf("expected one due reminder, got %v", due)
	}
	reminder := due[0].(map[string]any)
	reminderID := reminder["id"].(string)
	if reminder["kind"] != "period_upcoming" {
		t.Fatalf("reminder kind = %v", reminder["kind"])
	}
	check(200, "POST", "/care/reminders/receipts", alice, map[string]any{"id": reminderID})
	if len(check(200, "GET", "/care/reminders", alice, nil)["reminders"].([]any)) != 0 {
		t.Fatal("receipt did not hide the reminder on this device")
	}
	if len(check(200, "GET", "/care/reminders", aliceSecond, nil)["reminders"].([]any)) != 1 {
		t.Fatal("the second device lost the reminder")
	}
	check(200, "POST", "/care/reminders/receipts", alice, map[string]any{"id": reminderID})
	check(400, "POST", "/care/reminders/receipts", alice, map[string]any{"id": "not-a-uuid"})

	// A shared image exists before unbind so the caller can prove the
	// relationship check now rejects stale couple ids.
	png := base64.StdEncoding.EncodeToString(append([]byte("\x89PNG\r\n\x1a\n"), []byte("care integration image")...))
	imageID := check(200, "POST", "/images", alice, map[string]any{"couple_id": cid, "data": png})["id"].(string)
	if code := rawStatus(baseURL + "/images/" + imageID + "?token=" + alice); code != 200 {
		t.Fatalf("image readable before unbind = %d", code)
	}
	check(200, "GET", "/notes?couple_id="+cid, bob, nil)
	return imageID
}

// careContains4 reports whether /account advertises the care capability.
func careContains4(data map[string]any) bool {
	features, ok := data["features"].([]any)
	if !ok {
		return false
	}
	for _, feature := range features {
		if feature == "cycle_care_v1" {
			return true
		}
	}
	return false
}
