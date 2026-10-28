package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type interaction struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Status    string    `json:"status"`
	Result    *string   `json:"result"`
}

func interactionTTL(action string) time.Duration {
	switch action {
	case "poke", "miss_you", "hug", "vibrate":
		return 120 * time.Second
	case "ring":
		return 30 * time.Second
	default:
		return 0
	}
}

func (s *Server) sendInteraction(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		ID       string `json:"id"`
		CoupleID string `json:"couple_id"`
		Action   string `json:"action"`
	}
	if err := decode(w, r, &input); err != nil {
		return err
	}
	ttl := interactionTTL(input.Action)
	if !uuidPattern.MatchString(input.ID) || ttl == 0 {
		return fail(400, "invalid_interaction")
	}
	tx, id, peer, err := s.chatTransaction(r, input.CoupleID)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var existingCouple, existingSender, existingAction string
	err = tx.QueryRow(r.Context(), `SELECT couple_id::text,sender_id::text,action_id FROM interactions WHERE id=$1`, input.ID).Scan(&existingCouple, &existingSender, &existingAction)
	if err == nil {
		if existingCouple != input.CoupleID || existingSender != id.User.ID || existingAction != input.Action {
			return fail(409, "interaction_id_conflict")
		}
		write(w, 200, map[string]any{"id": input.ID, "status": "sent"})
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var limited bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM interactions WHERE sender_id=$1 AND created_at>now()-interval '1 second') OR ($2='ring' AND EXISTS(SELECT 1 FROM interactions WHERE sender_id=$1 AND action_id='ring' AND created_at>now()-interval '15 seconds'))`, id.User.ID, input.Action).Scan(&limited)
	if err != nil {
		return err
	}
	if limited {
		return fail(429, "interaction_rate_limited")
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO interactions(id,couple_id,sender_id,recipient_id,action_id,expires_at) VALUES($1,$2,$3,$4,$5,now()+$6 * interval '1 second')`, input.ID, input.CoupleID, id.User.ID, peer, input.Action, int(ttl.Seconds()))
	if err != nil {
		return err
	}
	if err = event(r.Context(), tx, "interaction.created", input.ID, peer); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 201, map[string]any{"id": input.ID, "status": "sent"})
	return nil
}

func (s *Server) listInteractions(w http.ResponseWriter, r *http.Request) error {
	couple := r.URL.Query().Get("couple_id")
	tx, id, _, err := s.chatTransaction(r, couple)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	rows, err := tx.Query(r.Context(), `SELECT id::text,action_id,created_at,expires_at,CASE WHEN result IS NOT NULL THEN 'finished' WHEN claimed_at IS NOT NULL THEN 'received' WHEN expires_at<=now() THEN 'expired' ELSE 'sent' END,result FROM interactions WHERE couple_id=$1 AND sender_id=$2 ORDER BY created_at DESC LIMIT 20`, couple, id.User.ID)
	if err != nil {
		return err
	}
	items := []interaction{}
	for rows.Next() {
		var item interaction
		if err = rows.Scan(&item.ID, &item.Action, &item.CreatedAt, &item.ExpiresAt, &item.Status, &item.Result); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]any{"interactions": items})
	return nil
}

func (s *Server) claimInteraction(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		CoupleID string `json:"couple_id"`
	}
	if err := decode(w, r, &input); err != nil {
		return err
	}
	tx, id, peer, err := s.chatTransaction(r, input.CoupleID)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var item interaction
	var remaining float64
	err = tx.QueryRow(r.Context(), `SELECT id::text,action_id,created_at,expires_at,extract(epoch from expires_at-clock_timestamp()) FROM interactions WHERE couple_id=$1 AND recipient_id=$2 AND claimed_at IS NULL AND expires_at>clock_timestamp() ORDER BY created_at LIMIT 1 FOR UPDATE`, input.CoupleID, id.User.ID).Scan(&item.ID, &item.Action, &item.CreatedAt, &item.ExpiresAt, &remaining)
	if errors.Is(err, pgx.ErrNoRows) {
		write(w, 200, map[string]any{"interaction": nil})
		return nil
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(r.Context(), `UPDATE interactions SET claimed_device=$2,claimed_at=now() WHERE id=$1`, item.ID, id.DeviceID)
	if err != nil {
		return err
	}
	if err = event(r.Context(), tx, "interaction.received", item.ID, peer); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	item.Status = "received"
	write(w, 200, map[string]any{"interaction": item, "remaining_ms": int64(remaining * 1000)})
	return nil
}

func (s *Server) interactionResult(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		CoupleID string `json:"couple_id"`
		ID       string `json:"id"`
		Result   string `json:"result"`
	}
	if err := decode(w, r, &input); err != nil {
		return err
	}
	if !uuidPattern.MatchString(input.ID) || (input.Result != "requested" && input.Result != "suppressed" && input.Result != "unsupported" && input.Result != "failed") {
		return fail(400, "invalid_interaction")
	}
	tx, id, peer, err := s.chatTransaction(r, input.CoupleID)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	tag, err := tx.Exec(r.Context(), `UPDATE interactions SET result=$1,result_at=now() WHERE id=$2 AND couple_id=$3 AND recipient_id=$4 AND claimed_device=$5 AND result IS NULL`, input.Result, input.ID, input.CoupleID, id.User.ID, id.DeviceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fail(409, "interaction_not_claimed")
	}
	if err = event(r.Context(), tx, "interaction.result", input.ID, peer); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]bool{"ok": true})
	return nil
}
