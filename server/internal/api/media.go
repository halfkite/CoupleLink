package api

import (
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const maxImageBytes = 6 << 20

// detectImage accepts only the three formats the client can render, matched by
// their own magic bytes so a declared MIME cannot smuggle anything else in.
func detectImage(raw []byte) string {
	switch {
	case len(raw) > 3 && raw[0] == 0xff && raw[1] == 0xd8 && raw[2] == 0xff:
		return "image/jpeg"
	case len(raw) > 8 && string(raw[:8]) == "\x89PNG\r\n\x1a\n":
		return "image/png"
	case len(raw) > 12 && string(raw[:4]) == "RIFF" && string(raw[8:12]) == "WEBP":
		return "image/webp"
	}
	return ""
}

func (s *Server) coupleMember(r *http.Request, coupleID string) (identity, error) {
	id, err := s.auth(r)
	if err != nil {
		return id, err
	}
	if !uuidPattern.MatchString(coupleID) {
		return id, fail(400, "invalid_input")
	}
	member, err := s.isMember(r, coupleID, id.User.ID)
	if err != nil {
		return id, err
	}
	if !member {
		return id, fail(403, "relationship_changed")
	}
	return id, nil
}

func (s *Server) isMember(r *http.Request, coupleID, userID string) (bool, error) {
	var a, b string
	err := s.db.QueryRow(r.Context(), `SELECT user_a::text,user_b::text FROM couples WHERE id=$1`, coupleID).Scan(&a, &b)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return userID == a || userID == b, nil
}

// authToken authenticates a session passed in the query string; <img> tags cannot
// send an Authorization header.
func (s *Server) authToken(r *http.Request, token string) (identity, error) {
	if len(token) != 43 {
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

func (s *Server) uploadImage(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		CoupleID string `json:"couple_id"`
		Data     string `json:"data"`
	}
	if err := decodeLimit(w, r, &input, 12<<20); err != nil {
		return err
	}
	id, err := s.coupleMember(r, input.CoupleID)
	if err != nil {
		return err
	}
	raw, err := base64.StdEncoding.DecodeString(input.Data)
	if err != nil || len(raw) == 0 {
		return fail(400, "invalid_image")
	}
	if len(raw) > maxImageBytes {
		return fail(413, "image_too_large")
	}
	mime := detectImage(raw)
	if mime == "" {
		return fail(400, "invalid_image")
	}
	if s.cfg.DataDir == "" {
		return fail(503, "storage_unavailable")
	}
	imageID, err := identifier()
	if err != nil {
		return err
	}
	dir := filepath.Join(s.cfg.DataDir, "images", input.CoupleID)
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return fail(503, "storage_unavailable")
	}
	if err = os.WriteFile(filepath.Join(dir, imageID), raw, 0o600); err != nil {
		return fail(503, "storage_unavailable")
	}
	if _, err = s.db.Exec(r.Context(), `INSERT INTO images(id,couple_id,uploader_id,mime,bytes) VALUES($1,$2,$3,$4,$5)`, imageID, input.CoupleID, id.User.ID, mime, len(raw)); err != nil {
		return err
	}
	write(w, 200, map[string]any{"id": imageID, "mime": mime, "bytes": len(raw)})
	return nil
}

func (s *Server) serveImage(w http.ResponseWriter, r *http.Request) error {
	imageID := r.PathValue("id")
	id, err := s.authToken(r, r.URL.Query().Get("token"))
	if err != nil {
		return err
	}
	if !uuidPattern.MatchString(imageID) || s.cfg.DataDir == "" {
		return fail(404, "not_found")
	}
	var coupleID, mime string
	err = s.db.QueryRow(r.Context(), `SELECT couple_id::text,mime FROM images WHERE id=$1`, imageID).Scan(&coupleID, &mime)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(404, "not_found")
	}
	if err != nil {
		return err
	}
	member, err := s.isMember(r, coupleID, id.User.ID)
	if err != nil {
		return err
	}
	if !member {
		return fail(403, "relationship_changed")
	}
	file, err := os.Open(filepath.Join(s.cfg.DataDir, "images", coupleID, imageID))
	if err != nil {
		return fail(404, "not_found")
	}
	defer file.Close()
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	http.ServeContent(w, r, imageID, time.Time{}, file)
	return nil
}

type Note struct {
	ID        string    `json:"id"`
	CoupleID  string    `json:"couple_id"`
	AuthorID  string    `json:"author_id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	ImageID   *string   `json:"image_id"`
	Featured  bool      `json:"featured"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const noteColumns = `id::text,couple_id::text,author_id::text,title,body,image_id::text,featured,created_at,updated_at`

func scanNote(row pgx.Row) (Note, error) {
	var n Note
	err := row.Scan(&n.ID, &n.CoupleID, &n.AuthorID, &n.Title, &n.Body, &n.ImageID, &n.Featured, &n.CreatedAt, &n.UpdatedAt)
	return n, err
}

func (s *Server) listNotes(w http.ResponseWriter, r *http.Request) error {
	coupleID := r.URL.Query().Get("couple_id")
	if _, err := s.coupleMember(r, coupleID); err != nil {
		return err
	}
	rows, err := s.db.Query(r.Context(), `SELECT `+noteColumns+` FROM notes WHERE couple_id=$1 ORDER BY created_at DESC LIMIT 200`, coupleID)
	if err != nil {
		return err
	}
	defer rows.Close()
	notes := []Note{}
	for rows.Next() {
		note, scanErr := scanNote(rows)
		if scanErr != nil {
			return scanErr
		}
		notes = append(notes, note)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	write(w, 200, map[string]any{"notes": notes})
	return nil
}

func validNoteText(value string, max int) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, 0) && utf8.RuneCountInString(value) <= max
}

func (s *Server) noteImage(r *http.Request, coupleID, imageID string) (*string, error) {
	if imageID == "" {
		return nil, nil
	}
	if !uuidPattern.MatchString(imageID) {
		return nil, fail(400, "invalid_image")
	}
	var owner string
	err := s.db.QueryRow(r.Context(), `SELECT couple_id::text FROM images WHERE id=$1`, imageID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != coupleID) {
		return nil, fail(400, "invalid_image")
	}
	if err != nil {
		return nil, err
	}
	return &imageID, nil
}

func (s *Server) createNote(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		CoupleID string `json:"couple_id"`
		Title    string `json:"title"`
		Body     string `json:"body"`
		ImageID  string `json:"image_id"`
	}
	if err := decodeLimit(w, r, &input, 64<<10); err != nil {
		return err
	}
	id, err := s.coupleMember(r, input.CoupleID)
	if err != nil {
		return err
	}
	input.Title = strings.TrimSpace(input.Title)
	if !validNoteText(input.Title, 120) || !validNoteText(input.Body, 4000) || (input.Title == "" && strings.TrimSpace(input.Body) == "" && input.ImageID == "") {
		return fail(400, "invalid_note")
	}
	imageID, err := s.noteImage(r, input.CoupleID, input.ImageID)
	if err != nil {
		return err
	}
	noteID, err := identifier()
	if err != nil {
		return err
	}
	note, err := scanNote(s.db.QueryRow(r.Context(), `INSERT INTO notes(id,couple_id,author_id,title,body,image_id) VALUES($1,$2,$3,$4,$5,$6) RETURNING `+noteColumns, noteID, input.CoupleID, id.User.ID, input.Title, input.Body, imageID))
	if err != nil {
		return err
	}
	write(w, 201, map[string]any{"note": note})
	return nil
}

func (s *Server) updateNote(w http.ResponseWriter, r *http.Request) error {
	noteID := r.PathValue("id")
	if !uuidPattern.MatchString(noteID) {
		return fail(400, "invalid_input")
	}
	var input struct {
		CoupleID string `json:"couple_id"`
		Title    string `json:"title"`
		Body     string `json:"body"`
		ImageID  string `json:"image_id"`
	}
	if err := decodeLimit(w, r, &input, 64<<10); err != nil {
		return err
	}
	if _, err := s.coupleMember(r, input.CoupleID); err != nil {
		return err
	}
	input.Title = strings.TrimSpace(input.Title)
	if !validNoteText(input.Title, 120) || !validNoteText(input.Body, 4000) {
		return fail(400, "invalid_note")
	}
	imageID, err := s.noteImage(r, input.CoupleID, input.ImageID)
	if err != nil {
		return err
	}
	note, err := scanNote(s.db.QueryRow(r.Context(), `UPDATE notes SET title=$3,body=$4,image_id=$5,updated_at=now() WHERE id=$1 AND couple_id=$2 RETURNING `+noteColumns, noteID, input.CoupleID, input.Title, input.Body, imageID))
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(404, "note_not_found")
	}
	if err != nil {
		return err
	}
	write(w, 200, map[string]any{"note": note})
	return nil
}

func (s *Server) setNoteFeatured(w http.ResponseWriter, r *http.Request) error {
	noteID := r.PathValue("id")
	if !uuidPattern.MatchString(noteID) {
		return fail(400, "invalid_input")
	}
	var input struct {
		CoupleID string `json:"couple_id"`
		Featured bool   `json:"featured"`
	}
	if err := decodeLimit(w, r, &input, 4<<10); err != nil {
		return err
	}
	if _, err := s.coupleMember(r, input.CoupleID); err != nil {
		return err
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if input.Featured {
		if _, err = tx.Exec(r.Context(), `UPDATE notes SET featured=false WHERE couple_id=$1 AND featured`, input.CoupleID); err != nil {
			return err
		}
	}
	note, err := scanNote(tx.QueryRow(r.Context(), `UPDATE notes SET featured=$3, updated_at=now() WHERE id=$1 AND couple_id=$2 RETURNING `+noteColumns, noteID, input.CoupleID, input.Featured))
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(404, "note_not_found")
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]any{"note": note})
	return nil
}
func (s *Server) deleteNote(w http.ResponseWriter, r *http.Request) error {
	noteID := r.PathValue("id")
	coupleID := r.URL.Query().Get("couple_id")
	if !uuidPattern.MatchString(noteID) {
		return fail(400, "invalid_input")
	}
	if _, err := s.coupleMember(r, coupleID); err != nil {
		return err
	}
	tag, err := s.db.Exec(r.Context(), `DELETE FROM notes WHERE id=$1 AND couple_id=$2`, noteID, coupleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fail(404, "note_not_found")
	}
	write(w, 200, map[string]bool{"ok": true})
	return nil
}
