package api

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestAnniversaryCalendarBoundaries(t *testing.T) {
	cases := []struct{ date, clock, repeat, zone, now, want string }{
		{"2024-02-29", "09:00", "yearly", "Asia/Shanghai", "2025-02-27T18:00:00Z", "2025-02-28T01:00:00Z"},
		{"2024-02-29", "09:00", "yearly", "Asia/Shanghai", "2025-02-28T01:00:00Z", "2026-02-28T01:00:00Z"},
		{"2024-02-29", "09:00", "yearly", "UTC", "2027-03-01T00:00:00Z", "2028-02-29T09:00:00Z"},
		{"2025-01-01", "09:00", "none", "UTC", "2026-01-01T00:00:00Z", ""},
		{"2026-12-01", "", "yearly", "UTC", "2026-01-01T00:00:00Z", ""},
	}
	for _, c := range cases {
		now, _ := time.Parse(time.RFC3339, c.now)
		got, err := nextAnniversaryReminder(c.date, c.clock, c.repeat, c.zone, now)
		if err != nil {
			t.Fatal(err)
		}
		if c.want == "" {
			if got != nil {
				t.Fatal("disabled or past one-time reminder scheduled")
			}
		} else if got == nil || got.UTC().Format(time.RFC3339) != c.want {
			t.Fatalf("%+v got %v", c, got)
		}
	}
	for _, c := range []struct{ date, clock, repeat, zone string }{{"2025-02-29", "09:00", "yearly", "UTC"}, {"0000-01-01", "09:00", "none", "UTC"}, {"2026-01-01", "25:00", "yearly", "UTC"}, {"2026-01-01", "09:00", "monthly", "UTC"}, {"2026-01-01", "09:00", "none", "Mars"}} {
		if _, err := nextAnniversaryReminder(c.date, c.clock, c.repeat, c.zone, time.Now()); err == nil {
			t.Fatalf("accepted invalid %+v", c)
		}
	}
}
func testMemories(t *testing.T, ctx context.Context, s *Server, db *pgxpool.Pool, call func(string, string, string, any) (int, map[string]any), alice, bob, third string, couple map[string]any) {
	t.Helper()
	cid := couple["id"].(string)
	check := func(want int, method, path, token string, body any) map[string]any {
		t.Helper()
		status, data := call(method, path, token, body)
		if status != want {
			t.Fatalf("%s %s status %d code %v", method, path, status, data["code"])
		}
		return data
	}
	check(403, "PUT", "/couples/current", third, map[string]any{"couple_id": cid, "start_at": "2024-02-29", "version": couple["version"]})
	check(400, "PUT", "/couples/current", alice, map[string]any{"couple_id": cid, "start_at": "2025-02-29", "version": couple["version"]})
	check(200, "PUT", "/couples/current", alice, map[string]any{"couple_id": cid, "start_at": "2024-02-29", "version": couple["version"]})
	check(409, "PUT", "/couples/current", bob, map[string]any{"couple_id": cid, "start_at": "2024-01-01", "version": couple["version"]})
	current := check(200, "GET", "/couples/current", bob, nil)["couple"].(map[string]any)
	if current["start_at"] != "2024-02-29" {
		t.Fatal("date did not sync")
	}
	couple["version"] = current["version"]
	id, _ := identifier()
	input := map[string]any{"id": id, "couple_id": cid, "title": "纪念日", "date": "2024-02-29", "recurrence": "yearly", "reminder_time": "09:00", "version": 0}
	check(201, "POST", "/anniversaries", alice, input)
	check(200, "POST", "/anniversaries", alice, input)
	check(403, "GET", "/anniversaries?couple_id="+cid, third, nil)
	list := check(200, "GET", "/anniversaries?couple_id="+cid, bob, nil)["anniversaries"].([]any)
	if len(list) != 1 {
		t.Fatal("duplicate create")
	}
	if _, err := db.Exec(ctx, `UPDATE anniversaries SET next_reminder=now()-interval '1 minute' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if err := s.generateAnniversaryReminders(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.generateAnniversaryReminders(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	path := "/anniversaries/reminders?couple_id=" + cid
	due := check(200, "GET", path, bob, nil)["reminders"].([]any)
	if len(due) != 1 {
		t.Fatalf("reminder dedup failed %d", len(due))
	}
	rid := due[0].(map[string]any)["id"]
	check(403, "POST", "/anniversaries/reminders/receipts", third, map[string]any{"couple_id": cid, "id": rid})
	check(200, "POST", "/anniversaries/reminders/receipts", bob, map[string]any{"couple_id": cid, "id": rid})
	if len(check(200, "GET", path, bob, nil)["reminders"].([]any)) != 0 {
		t.Fatal("receipt did not dedup")
	}
	if len(check(200, "GET", path, alice, nil)["reminders"].([]any)) != 1 {
		t.Fatal("other device lost reminder")
	}
	input["version"] = 1
	input["reminder_time"] = ""
	check(200, "PUT", "/anniversaries/"+id, alice, input)
	check(409, "PUT", "/anniversaries/"+id, bob, input)
	if len(check(200, "GET", path, alice, nil)["reminders"].([]any)) != 0 {
		t.Fatal("old reminder survived edit")
	}
	check(200, "DELETE", "/anniversaries/"+id, bob, map[string]any{"couple_id": cid, "version": 2})
	if len(check(200, "GET", "/anniversaries?couple_id="+cid, alice, nil)["anniversaries"].([]any)) != 0 {
		t.Fatal("deleted anniversary visible")
	}
	base := "/messages/search?couple_id=" + cid + "&q="
	check(403, "GET", base+"fixture", third, nil)
	check(400, "GET", base, alice, nil)
	page := check(200, "GET", base+"fixture", bob, nil)
	if len(page["messages"].([]any)) != 50 || page["has_more"] != true {
		t.Fatal("search page boundary")
	}
	next := page["next_before"].(float64)
	page2 := check(200, "GET", base+"fixture&before="+strconv.FormatInt(int64(next), 10), bob, nil)
	if len(page2["messages"].([]any)) != 49 || page2["has_more"] != false {
		t.Fatal("search pagination")
	}
	if len(check(200, "GET", base+url.QueryEscape("真实消息"), bob, nil)["messages"].([]any)) != 1 {
		t.Fatal("Chinese search")
	}
	if len(check(200, "GET", base+url.QueryEscape("%_\\"), bob, nil)["messages"].([]any)) != 0 {
		t.Fatal("wildcards interpreted")
	}
	contextPage := check(200, "GET", "/messages/context?couple_id="+cid+"&seq=50", bob, nil)
	if len(contextPage["messages"].([]any)) != 41 {
		t.Fatal("context boundary")
	}
	check(404, "GET", "/messages/context?couple_id="+cid+"&seq=999", bob, nil)
	check(403, "GET", "/messages/context?couple_id="+cid+"&seq=50", third, nil)
}
