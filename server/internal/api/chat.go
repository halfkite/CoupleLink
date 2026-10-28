package api

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const messagePageSize = 100

type ChatMessage struct {
	ID        string    `json:"id"`
	CoupleID  string    `json:"couple_id"`
	SenderID  string    `json:"sender_id"`
	Body      string    `json:"body"`
	Seq       int64     `json:"seq"`
	CreatedAt time.Time `json:"created_at"`
	Delivered bool      `json:"delivered"`
}

const messageColumns = `m.id::text,m.couple_id::text,m.sender_id::text,m.body,m.seq,m.created_at,
 EXISTS(SELECT 1 FROM message_receipts r WHERE r.message_id=m.id AND r.user_id<>m.sender_id)`

func scanMessage(row pgx.Row) (ChatMessage, error) {
	var m ChatMessage
	err := row.Scan(&m.ID, &m.CoupleID, &m.SenderID, &m.Body, &m.Seq, &m.CreatedAt, &m.Delivered)
	return m, err
}

// All chat operations lock the same sorted user rows as unbind. A request
// rechecks both session and active membership after taking those locks.
func (s *Server) chatTransaction(r *http.Request, coupleID string) (pgx.Tx, identity, string, error) {
	id, err := s.auth(r)
	if err != nil {
		return nil, id, "", err
	}
	if !uuidPattern.MatchString(coupleID) {
		return nil, id, "", fail(400, "invalid_input")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return nil, id, "", err
	}
	ok := false
	defer func() {
		if !ok {
			tx.Rollback(r.Context())
		}
	}()
	var a, b string
	err = tx.QueryRow(r.Context(), `SELECT user_a::text,user_b::text FROM couples WHERE id=$1`, coupleID).Scan(&a, &b)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, id, "", fail(403, "relationship_changed")
	}
	if err != nil {
		return nil, id, "", err
	}
	if id.User.ID != a && id.User.ID != b {
		return nil, id, "", fail(403, "relationship_changed")
	}
	if err = lockUsers(r.Context(), tx, a, b); err != nil {
		return nil, id, "", err
	}
	var sessionValid bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM sessions s JOIN devices d ON d.id=s.device_id WHERE s.id=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND d.revoked_at IS NULL)`, id.SessionID).Scan(&sessionValid)
	if err != nil {
		return nil, id, "", err
	}
	if !sessionValid {
		return nil, id, "", fail(401, "unauthorized")
	}
	var authorized bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM couple_members cm JOIN couples c ON c.id=cm.couple_id
 JOIN sessions s ON s.id=$3 AND s.user_id=cm.user_id JOIN devices d ON d.id=s.device_id
 WHERE cm.user_id=$1 AND cm.couple_id=$2 AND c.status='active'
 AND s.revoked_at IS NULL AND s.expires_at>now() AND d.revoked_at IS NULL)`, id.User.ID, coupleID, id.SessionID).Scan(&authorized)
	if err != nil {
		return nil, id, "", err
	}
	if !authorized {
		return nil, id, "", fail(403, "relationship_changed")
	}
	peer := a
	if peer == id.User.ID {
		peer = b
	}
	ok = true
	return tx, id, peer, nil
}
func (s *Server) sendMessage(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		ID       string `json:"id"`
		CoupleID string `json:"couple_id"`
		Body     string `json:"body"`
	}
	if err := decodeLimit(w, r, &input, 32*1024); err != nil {
		return err
	}
	if !uuidPattern.MatchString(input.ID) || !utf8.ValidString(input.Body) || strings.TrimSpace(input.Body) == "" || utf8.RuneCountInString(input.Body) > 4000 || strings.ContainsRune(input.Body, 0) {
		return fail(400, "invalid_message")
	}
	tx, id, peer, err := s.chatTransaction(r, input.CoupleID)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	previous, err := scanMessage(tx.QueryRow(r.Context(), `SELECT `+messageColumns+` FROM messages m WHERE m.id=$1`, input.ID))
	if err == nil {
		if previous.SenderID != id.User.ID || previous.CoupleID != input.CoupleID || previous.Body != input.Body {
			return fail(409, "message_id_conflict")
		}
		write(w, 200, map[string]any{"message": previous})
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var seq int64
	if err = tx.QueryRow(r.Context(), `UPDATE couples SET message_seq=message_seq+1 WHERE id=$1 RETURNING message_seq`, input.CoupleID).Scan(&seq); err != nil {
		return err
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO messages(id,couple_id,sender_id,body,seq) VALUES($1,$2,$3,$4,$5)`, input.ID, input.CoupleID, id.User.ID, input.Body, seq)
	if err != nil {
		return err
	}
	if err = event(r.Context(), tx, "message.created", input.ID, id.User.ID, peer); err != nil {
		return err
	}
	message, err := scanMessage(tx.QueryRow(r.Context(), `SELECT `+messageColumns+` FROM messages m WHERE m.id=$1`, input.ID))
	if err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 201, map[string]any{"message": message})
	return nil
}
func (s *Server) listMessages(w http.ResponseWriter, r *http.Request) error {
	coupleID := r.URL.Query().Get("couple_id")
	after, before := int64(0), int64(0)
	var err error
	if value := r.URL.Query().Get("after"); value != "" {
		after, err = strconv.ParseInt(value, 10, 64)
		if err != nil || after < 0 {
			return fail(400, "invalid_cursor")
		}
	}
	if value := r.URL.Query().Get("before"); value != "" {
		before, err = strconv.ParseInt(value, 10, 64)
		if err != nil || before < 1 || after > 0 {
			return fail(400, "invalid_cursor")
		}
	}
	tx, _, peer, err := s.chatTransaction(r, coupleID)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var head int64
	if err = tx.QueryRow(r.Context(), `SELECT message_seq FROM couples WHERE id=$1`, coupleID).Scan(&head); err != nil {
		return err
	}
	if after > head || before > head+1 {
		return fail(400, "invalid_cursor")
	}
	query := `SELECT ` + messageColumns + ` FROM messages m WHERE m.couple_id=$1 AND m.seq>$2 ORDER BY m.seq LIMIT 100`
	bound := after
	if before > 0 {
		query = `SELECT ` + messageColumns + ` FROM messages m WHERE m.couple_id=$1 AND m.seq<$2 ORDER BY m.seq DESC LIMIT 100`
		bound = before
	}
	rows, err := tx.Query(r.Context(), query, coupleID, bound)
	if err != nil {
		return err
	}
	messages := []ChatMessage{}
	for rows.Next() {
		message, scanErr := scanMessage(rows)
		if scanErr != nil {
			rows.Close()
			return scanErr
		}
		messages = append(messages, message)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	if before > 0 {
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
	}
	next := after
	if len(messages) > 0 {
		next = messages[len(messages)-1].Seq
	}
	var delivered int64
	if err = tx.QueryRow(r.Context(), `SELECT COALESCE(max(m.seq),0) FROM message_receipts r JOIN messages m ON m.id=r.message_id WHERE m.couple_id=$1 AND r.user_id=$2`, coupleID, peer).Scan(&delivered); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]any{"messages": messages, "head": head, "next_cursor": next, "peer_delivered_through": delivered, "has_more": len(messages) == messagePageSize})
	return nil
}
func (s *Server) deliverMessages(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		CoupleID string `json:"couple_id"`
		Through  int64  `json:"through"`
	}
	if err := decode(w, r, &input); err != nil {
		return err
	}
	if input.Through < 1 {
		return fail(400, "invalid_cursor")
	}
	tx, id, peer, err := s.chatTransaction(r, input.CoupleID)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var head int64
	if err = tx.QueryRow(r.Context(), `SELECT message_seq FROM couples WHERE id=$1`, input.CoupleID).Scan(&head); err != nil {
		return err
	}
	if input.Through > head {
		return fail(400, "invalid_cursor")
	}
	tag, err := tx.Exec(r.Context(), `INSERT INTO message_receipts(message_id,device_id,user_id)
 SELECT id,$1,$2 FROM messages WHERE couple_id=$3 AND seq<=$4 AND sender_id<>$2 ON CONFLICT DO NOTHING`, id.DeviceID, id.User.ID, input.CoupleID, input.Through)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		if err = event(r.Context(), tx, "message.delivered", input.CoupleID, id.User.ID, peer); err != nil {
			return err
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]bool{"ok": true})
	return nil
}
func enqueueRealtime(ctx context.Context, tx pgx.Tx, kind, target string) error {
	id, err := identifier()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO realtime_outbox(id,kind,target) VALUES($1,$2,$3)`, id, kind, target)
	return err
}

func revokeSessions(ctx context.Context, tx pgx.Tx, user, device string) error {
	rows, err := tx.Query(ctx, `UPDATE sessions SET revoked_at=now() WHERE user_id=$1 AND device_id=$2 AND revoked_at IS NULL RETURNING id::text`, user, device)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err = enqueueRealtime(ctx, tx, "disconnect", id); err != nil {
			return err
		}
	}
	return nil
}
