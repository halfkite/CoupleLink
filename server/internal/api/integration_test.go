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
	status, data = call("DELETE", "/couples/current", alice, map[string]any{"version": 999})
	expect(409, status, data)
	status, data = call("DELETE", "/couples/current", alice, map[string]any{"version": couple["version"]})
	expect(200, status, data)
	status, data = call("GET", "/couples/current", bob, nil)
	expect(200, status, data)
	if data["couple"] != nil {
		t.Fatal("partner retained revoked relationship")
	}
	status, data = call("POST", "/couples/join", bob, join)
	expect(400, status, data)
	status, data = call("GET", "/sync/changes?cursor=1", bob, nil)
	expect(200, status, data)
	if data["events"].([]any)[0].(map[string]any)["type"] != "couple.unbound" {
		t.Fatal("missing unbind event")
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
	t.Log(fmt.Sprintf("real PostgreSQL verified: migration, no account cap or setup code, duplicate registration, concurrent single-use pairing, revisions, scoped sync, multi-device revocation"))
}
