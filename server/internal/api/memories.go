package api

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Anniversary struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Date         string     `json:"date"`
	Recurrence   string     `json:"recurrence"`
	ReminderTime string     `json:"reminder_time"`
	Timezone     string     `json:"timezone"`
	Version      int64      `json:"version"`
	NextReminder *time.Time `json:"next_reminder"`
}

const anniversaryColumns = `a.id::text,a.title,a.date::text,a.recurrence,a.reminder_time,a.timezone,a.version,a.next_reminder`

func scanAnniversary(row pgx.Row) (Anniversary, error) {
	var a Anniversary
	err := row.Scan(&a.ID, &a.Title, &a.Date, &a.Recurrence, &a.ReminderTime, &a.Timezone, &a.Version, &a.NextReminder)
	return a, err
}

// Date-only anniversaries retain their original month/day. February 29 is
// observed on February 28 in non-leap years; never drift the stored date.
func nextAnniversaryReminder(date, clock, recurrence, timezone string, after time.Time) (*time.Time, error) {
	d, err := time.Parse(time.DateOnly, date)
	if err != nil || d.Year() < 1 {
		return nil, fail(400, "invalid_input")
	}
	zone, err := time.LoadLocation(timezone)
	if err != nil || timezone == "" {
		return nil, fail(400, "invalid_input")
	}
	if recurrence != "none" && recurrence != "yearly" {
		return nil, fail(400, "invalid_input")
	}
	if clock == "" {
		return nil, nil
	}
	c, err := time.Parse("15:04", clock)
	if err != nil || c.Format("15:04") != clock {
		return nil, fail(400, "invalid_input")
	}
	local := after.In(zone)
	year := d.Year()
	if recurrence == "yearly" && local.Year() > year {
		year = local.Year()
	}
	for i := 0; i < 2; i++ {
		day := d.Day()
		max := time.Date(year, d.Month()+1, 0, 0, 0, 0, 0, zone).Day()
		if day > max {
			day = max
		}
		candidate := time.Date(year, d.Month(), day, c.Hour(), c.Minute(), 0, 0, zone)
		if candidate.After(after) {
			return &candidate, nil
		}
		if recurrence == "none" {
			return nil, nil
		}
		year++
	}
	return nil, fail(400, "invalid_input")
}
func (s *Server) updateCoupleDate(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		CoupleID string `json:"couple_id"`
		StartAt  string `json:"start_at"`
		Version  int64  `json:"version"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	tx, id, peer, err := s.chatTransaction(r, in.CoupleID)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var zoneName string
	if err = tx.QueryRow(r.Context(), `SELECT timezone FROM couples WHERE id=$1`, in.CoupleID).Scan(&zoneName); err != nil {
		return err
	}
	zone, err := time.LoadLocation(zoneName)
	if err != nil {
		return err
	}
	d, err := time.Parse(time.DateOnly, in.StartAt)
	if err != nil || d.Year() < 1 || in.StartAt > time.Now().In(zone).Format(time.DateOnly) {
		return fail(400, "invalid_input")
	}
	tag, err := tx.Exec(r.Context(), `UPDATE couples SET start_at=$1,version=version+1 WHERE id=$2 AND version=$3`, d, in.CoupleID, in.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fail(409, "version_conflict")
	}
	if err = event(r.Context(), tx, "couple.updated", in.CoupleID, id.User.ID, peer); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]bool{"ok": true})
	return nil
}
func (s *Server) listAnniversaries(w http.ResponseWriter, r *http.Request) error {
	tx, _, _, err := s.chatTransaction(r, r.URL.Query().Get("couple_id"))
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	rows, err := tx.Query(r.Context(), `SELECT `+anniversaryColumns+` FROM anniversaries a WHERE couple_id=$1 AND deleted_at IS NULL ORDER BY date,id`, r.URL.Query().Get("couple_id"))
	if err != nil {
		return err
	}
	items := []Anniversary{}
	for rows.Next() {
		a, e := scanAnniversary(rows)
		if e != nil {
			rows.Close()
			return e
		}
		items = append(items, a)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]any{"anniversaries": items})
	return nil
}
func (s *Server) saveAnniversary(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		CoupleID     string `json:"couple_id"`
		Title        string `json:"title"`
		Date         string `json:"date"`
		Recurrence   string `json:"recurrence"`
		ReminderTime string `json:"reminder_time"`
		Version      int64  `json:"version"`
		ID           string `json:"id"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" || utf8.RuneCountInString(in.Title) > 80 || strings.ContainsRune(in.Title, 0) {
		return fail(400, "invalid_input")
	}
	id := r.PathValue("id")
	create := r.Method == "POST"
	if create {
		id = in.ID
	}
	if !uuidPattern.MatchString(id) {
		return fail(400, "invalid_input")
	}
	tx, actor, peer, err := s.chatTransaction(r, in.CoupleID)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var zone string
	if err = tx.QueryRow(r.Context(), `SELECT timezone FROM couples WHERE id=$1`, in.CoupleID).Scan(&zone); err != nil {
		return err
	}
	next, err := nextAnniversaryReminder(in.Date, in.ReminderTime, in.Recurrence, zone, time.Now())
	if err != nil {
		return err
	}
	if create {
		var count int
		if err = tx.QueryRow(r.Context(), `SELECT count(*) FROM anniversaries WHERE couple_id=$1 AND deleted_at IS NULL`, in.CoupleID).Scan(&count); err != nil {
			return err
		}
		if count >= 200 {
			return fail(400, "anniversary_limit")
		}
		// Stable client IDs make an interrupted create safe to retry without duplication.
		existing, e := scanAnniversary(tx.QueryRow(r.Context(), `SELECT `+anniversaryColumns+` FROM anniversaries a WHERE id=$1 AND couple_id=$2 AND deleted_at IS NULL`, id, in.CoupleID))
		if e == nil {
			if existing.Title != in.Title || existing.Date != in.Date || existing.Recurrence != in.Recurrence || existing.ReminderTime != in.ReminderTime {
				return fail(409, "conflict")
			}
			write(w, 200, map[string]any{"anniversary": existing})
			return nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		_, err = tx.Exec(r.Context(), `INSERT INTO anniversaries(id,couple_id,title,date,recurrence,reminder_time,timezone,next_reminder) VALUES($1,$2,$3,$4::date,$5,$6,$7,$8)`, id, in.CoupleID, in.Title, in.Date, in.Recurrence, in.ReminderTime, zone, next)
	} else {
		var tagRows int64
		tag, e := tx.Exec(r.Context(), `UPDATE anniversaries SET title=$1,date=$2::date,recurrence=$3,reminder_time=$4,next_reminder=$5,version=version+1 WHERE id=$6 AND couple_id=$7 AND version=$8 AND deleted_at IS NULL`, in.Title, in.Date, in.Recurrence, in.ReminderTime, next, id, in.CoupleID, in.Version)
		err = e
		tagRows = tag.RowsAffected()
		if err == nil && tagRows != 1 {
			return fail(409, "version_conflict")
		}
	}
	if err != nil {
		return err
	}
	if err = event(r.Context(), tx, "anniversary.updated", id, actor.User.ID, peer); err != nil {
		return err
	}
	a, err := scanAnniversary(tx.QueryRow(r.Context(), `SELECT `+anniversaryColumns+` FROM anniversaries a WHERE id=$1`, id))
	if err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	status := 200
	if create {
		status = 201
	}
	write(w, status, map[string]any{"anniversary": a})
	return nil
}
func (s *Server) deleteAnniversary(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		CoupleID string `json:"couple_id"`
		Version  int64  `json:"version"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		return fail(400, "invalid_input")
	}
	tx, actor, peer, err := s.chatTransaction(r, in.CoupleID)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	tag, err := tx.Exec(r.Context(), `UPDATE anniversaries SET deleted_at=now(),next_reminder=NULL,version=version+1 WHERE id=$1 AND couple_id=$2 AND version=$3 AND deleted_at IS NULL`, id, in.CoupleID, in.Version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fail(409, "version_conflict")
	}
	if err = event(r.Context(), tx, "anniversary.deleted", id, actor.User.ID, peer); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]bool{"ok": true})
	return nil
}
func (s *Server) generateAnniversaryReminders(ctx context.Context, now time.Time) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT `+anniversaryColumns+` FROM anniversaries a JOIN couples c ON c.id=a.couple_id WHERE a.deleted_at IS NULL AND c.status='active' AND a.next_reminder<=$1 ORDER BY a.next_reminder LIMIT 100 FOR UPDATE OF a SKIP LOCKED`, now)
	if err != nil {
		return err
	}
	due := []Anniversary{}
	for rows.Next() {
		a, e := scanAnniversary(rows)
		if e != nil {
			rows.Close()
			return e
		}
		due = append(due, a)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, a := range due {
		if now.Sub(*a.NextReminder) <= 24*time.Hour {
			id, e := identifier()
			if e != nil {
				return e
			}
			if _, err = tx.Exec(ctx, `INSERT INTO anniversary_reminders(id,anniversary_id,version,scheduled_at) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, id, a.ID, a.Version, a.NextReminder); err != nil {
				return err
			}
		}
		next, e := nextAnniversaryReminder(a.Date, a.ReminderTime, a.Recurrence, a.Timezone, now)
		if e != nil {
			return e
		}
		if _, err = tx.Exec(ctx, `UPDATE anniversaries SET next_reminder=$1 WHERE id=$2`, next, a.ID); err != nil {
			return err
		}
	}
	// Bound delivery metadata: older reminders are never delivered after 24h.
	if _, err = tx.Exec(ctx, `DELETE FROM anniversary_receipts WHERE reminder_id IN (SELECT id FROM anniversary_reminders WHERE scheduled_at<$1)`, now.Add(-24*time.Hour)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM anniversary_reminders WHERE scheduled_at<$1`, now.Add(-24*time.Hour)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Server) RunAnniversaryReminders(ctx context.Context) {
	timer := time.NewTicker(30 * time.Second)
	defer timer.Stop()
	for {
		run, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := s.generateAnniversaryReminders(run, time.Now())
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Print("anniversary reminder generation failed; durable rows retained for retry")
		}
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}
func (s *Server) pendingReminders(w http.ResponseWriter, r *http.Request) error {
	couple := r.URL.Query().Get("couple_id")
	tx, id, _, err := s.chatTransaction(r, couple)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	rows, err := tx.Query(r.Context(), `SELECT n.id::text,a.title,n.scheduled_at FROM anniversary_reminders n JOIN anniversaries a ON a.id=n.anniversary_id AND a.version=n.version WHERE a.couple_id=$1 AND a.deleted_at IS NULL AND n.scheduled_at BETWEEN now()-interval '24 hours' AND now() AND NOT EXISTS(SELECT 1 FROM anniversary_receipts r WHERE r.reminder_id=n.id AND r.device_id=$2) ORDER BY n.scheduled_at LIMIT 20`, couple, id.DeviceID)
	if err != nil {
		return err
	}
	items := []map[string]any{}
	for rows.Next() {
		var key, title string
		var at time.Time
		if err = rows.Scan(&key, &title, &at); err != nil {
			rows.Close()
			return err
		}
		items = append(items, map[string]any{"id": key, "title": title, "scheduled_at": at})
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]any{"reminders": items})
	return nil
}
func (s *Server) ackReminder(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		CoupleID string `json:"couple_id"`
		ID       string `json:"id"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if !uuidPattern.MatchString(in.ID) {
		return fail(400, "invalid_input")
	}
	tx, id, _, err := s.chatTransaction(r, in.CoupleID)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	tag, err := tx.Exec(r.Context(), `INSERT INTO anniversary_receipts(reminder_id,device_id) SELECT n.id,$1 FROM anniversary_reminders n JOIN anniversaries a ON a.id=n.anniversary_id AND a.version=n.version WHERE n.id=$2 AND a.couple_id=$3 AND a.deleted_at IS NULL ON CONFLICT DO NOTHING`, id.DeviceID, in.ID, in.CoupleID)
	if err != nil {
		return err
	}
	_ = tag
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]bool{"ok": true})
	return nil
}
func (s *Server) searchMessages(w http.ResponseWriter, r *http.Request) error {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || utf8.RuneCountInString(q) > 100 || strings.ContainsRune(q, 0) {
		return fail(400, "invalid_search")
	}
	before := int64(0)
	var err error
	if v := r.URL.Query().Get("before"); v != "" {
		before, err = strconv.ParseInt(v, 10, 64)
		if err != nil || before < 1 {
			return fail(400, "invalid_cursor")
		}
	}
	tx, _, _, err := s.chatTransaction(r, r.URL.Query().Get("couple_id"))
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	pattern := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
	rows, err := tx.Query(r.Context(), `SELECT `+messageColumns+` FROM messages m WHERE couple_id=$1 AND ($2::bigint=0 OR seq<$2) AND body ILIKE $3 ESCAPE '\' ORDER BY seq DESC LIMIT 51`, r.URL.Query().Get("couple_id"), before, pattern)
	if err != nil {
		return err
	}
	items := []ChatMessage{}
	for rows.Next() {
		m, e := scanMessage(rows)
		if e != nil {
			rows.Close()
			return e
		}
		items = append(items, m)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	more := len(items) > 50
	if more {
		items = items[:50]
	}
	next := int64(0)
	if len(items) > 0 {
		next = items[len(items)-1].Seq
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]any{"messages": items, "has_more": more, "next_before": next})
	return nil
}
func (s *Server) messageContext(w http.ResponseWriter, r *http.Request) error {
	seq, err := strconv.ParseInt(r.URL.Query().Get("seq"), 10, 64)
	if err != nil || seq < 1 {
		return fail(400, "invalid_cursor")
	}
	tx, _, _, err := s.chatTransaction(r, r.URL.Query().Get("couple_id"))
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var exists bool
	if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM messages WHERE couple_id=$1 AND seq=$2)`, r.URL.Query().Get("couple_id"), seq).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fail(404, "message_not_found")
	}
	rows, err := tx.Query(r.Context(), `SELECT `+messageColumns+` FROM messages m WHERE couple_id=$1 AND seq BETWEEN GREATEST(1,$2::bigint-20) AND $2::bigint+20 ORDER BY seq`, r.URL.Query().Get("couple_id"), seq)
	if err != nil {
		return err
	}
	items := []ChatMessage{}
	for rows.Next() {
		m, e := scanMessage(rows)
		if e != nil {
			rows.Close()
			return e
		}
		items = append(items, m)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]any{"messages": items})
	return nil
}
