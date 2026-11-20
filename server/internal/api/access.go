package api

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"unicode/utf8"
	"time"

	"github.com/jackc/pgx/v5"
)

// Access keys replace username/password accounts: the admin page creates one key
// per couple group, and the same key signs in both phones of that group.
var accessKeyPattern = regexpKey()

func regexpKey() *simplePattern {
	return &simplePattern{prefix: "key", alphabet: "abcdefghijklmnopqrstuvwxyz234567", min: 16, max: 32}
}

// simplePattern avoids pulling a regexp into every request path.
type simplePattern struct {
	prefix   string
	alphabet string
	min, max int
}

func (p *simplePattern) Match(value string) bool {
	if !strings.HasPrefix(value, p.prefix) {
		return false
	}
	body := value[len(p.prefix):]
	if len(body) < p.min || len(body) > p.max {
		return false
	}
	for _, r := range body {
		if !strings.ContainsRune(p.alphabet, r) {
			return false
		}
	}
	return true
}

func newAccessKey() (string, error) {
	raw := make([]byte, 13)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	encoded := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw))
	if len(encoded) < 20 {
		return "", errors.New("short key")
	}
	return "key" + encoded[:20], nil
}

// createGroupAndKey provisions a two-person group and one key per member.
// The plaintext is stored alongside the hash so the admin page can show it again.
func (s *Server) createGroupAndKey(ctx context.Context, label string) (string, string, error) {
	key, err := newAccessKey()
	if err != nil {
		return "", "", err
	}
	keyB, err := newAccessKey()
	if err != nil {
		return "", "", err
	}
	coupleID, err := identifier()
	if err != nil {
		return "", "", err
	}
	keyID, err := identifier()
	if err != nil {
		return "", "", err
	}
	keyBID, err := identifier()
	if err != nil {
		return "", "", err
	}
	userA, err := identifier()
	if err != nil {
		return "", "", err
	}
	userB, err := identifier()
	if err != nil {
		return "", "", err
	}
	unusable, err := randomToken()
	if err != nil {
		return "", "", err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)
	for index, id := range []string{userA, userB} {
		username := "group-" + strings.ReplaceAll(id, "-", "")[:16]
		display := fmt.Sprintf("成员 %s", []string{"A", "B"}[index])
		if _, err = tx.Exec(ctx, `INSERT INTO users(id,username,display_name,password_hash) VALUES($1,$2,$3,$4)`, id, username, display, tokenHash(unusable)); err != nil {
			return "", "", err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO sync_heads(user_id) VALUES($1)`, id); err != nil {
			return "", "", err
		}
	}
	start := time.Now().In(chinaZone()).Format("2006-01-02")
	if _, err = tx.Exec(ctx, `INSERT INTO couples(id,user_a,user_b,start_at,timezone,status) VALUES($1,$2,$3,$4,'Asia/Shanghai','active')`, coupleID, userA, userB, start); err != nil {
		return "", "", err
	}
	for _, id := range []string{userA, userB} {
		if _, err = tx.Exec(ctx, `INSERT INTO couple_members(user_id,couple_id) VALUES($1,$2)`, id, coupleID); err != nil {
			return "", "", err
		}
	}
	for _, entry := range []struct{ id, value, slot string }{{keyID, key, "a"}, {keyBID, keyB, "b"}} {
		hint := entry.value
		if len(hint) > 10 {
			hint = hint[:10]
		}
		if _, err = tx.Exec(ctx, `INSERT INTO access_keys(id,key_hash,key_plain,key_hint,couple_id,label,slot) VALUES($1,$2,$3,$4,$5,$6,$7)`, entry.id, tokenHash(entry.value), entry.value, hint, coupleID, label, entry.slot); err != nil {
			return "", "", err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return key, coupleID, nil
}

func chinaZone() *time.Location {
	zone, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return zone
}

type keyLoginInput struct {
	Key         string `json:"key"`
	Gender      string `json:"gender,omitempty"`
	DeviceID    string `json:"device_id"`
	DeviceLabel string `json:"device_label"`
	Platform    string `json:"platform"`
}

func validGender(value string) bool {
	return value == "" || value == "male" || value == "female"
}

// keyLogin signs a device in with the group's key and assigns it to the group
// member that has fewer active devices. Gender is a display label only.
func (s *Server) keyLogin(w http.ResponseWriter, r *http.Request) error {
	var input keyLoginInput
	if err := decode(w, r, &input); err != nil {
		return err
	}
	input.Key = strings.ToLower(strings.TrimSpace(input.Key))
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !s.limiter.allow(host) {
		return fail(429, "rate_limited")
	}
	if !accessKeyPattern.Match(input.Key) || !uuidPattern.MatchString(input.DeviceID) || !validText(input.DeviceLabel, 64) || !validText(input.Platform, 24) || !validGender(input.Gender) {
		return fail(400, "invalid_input")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var keyID, coupleID, userA, userB string
	var slot *string
	err = tx.QueryRow(r.Context(), `SELECT k.id::text,k.couple_id::text,c.user_a::text,c.user_b::text,k.slot
  FROM access_keys k JOIN couples c ON c.id=k.couple_id
  WHERE k.key_hash=$1 AND k.revoked_at IS NULL`, tokenHash(input.Key)).Scan(&keyID, &coupleID, &userA, &userB, &slot)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(401, "invalid_key")
	}
	if err != nil {
		return err
	}
	var countA, countB int
	var deviceA, deviceB bool
	if err = tx.QueryRow(r.Context(), `SELECT
  (SELECT count(*) FROM devices WHERE user_id=$1 AND revoked_at IS NULL),
  (SELECT count(*) FROM devices WHERE user_id=$2 AND revoked_at IS NULL),
  EXISTS(SELECT 1 FROM devices WHERE id=$3 AND user_id=$1),
  EXISTS(SELECT 1 FROM devices WHERE id=$3 AND user_id=$2)`, userA, userB, input.DeviceID).Scan(&countA, &countB, &deviceA, &deviceB); err != nil {
		return err
	}
	// A key issued for one slot always binds that member; legacy shared keys keep
	// the first-device/second-device behaviour.
	member := ""
	if slot != nil {
		switch *slot {
		case "a":
			member = userA
		case "b":
			member = userB
		}
	}
	if member == "" {
		member = userA
		switch {
		case deviceA:
			member = userA
		case deviceB:
			member = userB
		case countB < countA:
			member = userB
		}
	}
	if err = lockUsers(r.Context(), tx, member); err != nil {
		return err
	}
	if input.Gender != "" {
		if _, err = tx.Exec(r.Context(), `UPDATE users SET gender=$1 WHERE id=$2`, input.Gender, member); err != nil {
			return err
		}
	}
	token, expires, err := s.newSession(r.Context(), tx, member, authInput{DeviceID: input.DeviceID, DeviceLabel: input.DeviceLabel, Platform: input.Platform})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `UPDATE access_keys SET last_used_at=now() WHERE id=$1`, keyID); err != nil {
		return err
	}
	var user User
	if err = tx.QueryRow(r.Context(), `SELECT id::text,username,display_name,gender FROM users WHERE id=$1`, member).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Gender); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]any{"token": token, "expires_at": expires, "user": user, "couple_id": coupleID})
	return nil
}

// updateRemark saves the caller's private label for their partner. Both members
// and the admin page can read it, and it is included in backups.
func (s *Server) updateRemark(w http.ResponseWriter, r *http.Request) error {
	identityValue, err := s.auth(r)
	if err != nil {
		return err
	}
	var input struct {
		Remark string `json:"remark"`
	}
	if err := decode(w, r, &input); err != nil {
		return err
	}
	remark := strings.TrimSpace(input.Remark)
	if len([]rune(remark)) > 40 || !validOptionalText(remark) {
		return fail(400, "invalid_input")
	}
	var coupleID, userA string
	err = s.db.QueryRow(r.Context(), `SELECT c.id::text,c.user_a::text FROM couples c
  JOIN couple_members m ON m.couple_id=c.id AND m.user_id=$1 AND c.status='active' LIMIT 1`, identityValue.User.ID).Scan(&coupleID, &userA)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(409, "not_bound")
	}
	if err != nil {
		return err
	}
	column := "remark_b"
	if identityValue.User.ID == userA {
		column = "remark_a"
	}
	if _, err = s.db.Exec(r.Context(), `UPDATE couples SET `+column+`=$1 WHERE id=$2`, remark, coupleID); err != nil {
		return err
	}
	couple, err := s.readCouple(r.Context(), identityValue.User.ID)
	if err != nil {
		return err
	}
	write(w, 200, map[string]any{"couple": couple})
	return nil
}

// updateProfile lets a member set the nickname others see.
func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	var input struct {
		DisplayName string `json:"display_name"`
	}
	if err := decode(w, r, &input); err != nil {
		return err
	}
	name := strings.TrimSpace(input.DisplayName)
	if utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 24 || !validOptionalText(name) {
		return fail(400, "invalid_input")
	}
	if _, err = s.db.Exec(r.Context(), `UPDATE users SET display_name=$1 WHERE id=$2`, name, id.User.ID); err != nil {
		return err
	}
	write(w, 200, map[string]any{"user": map[string]string{"id": id.User.ID, "display_name": name}})
	return nil
}

// updateAvatar stores the picture a member picked; it must already be uploaded
// to the same couple so nobody can point at someone else's image.
func (s *Server) updateAvatar(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	var input struct {
		ImageID string `json:"image_id"`
	}
	if err := decode(w, r, &input); err != nil {
		return err
	}
	var imageID *string
	if input.ImageID != "" {
		if !uuidPattern.MatchString(input.ImageID) {
			return fail(400, "invalid_image")
		}
		var couple string
		var ok bool
		err = s.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM couple_members m JOIN images i ON i.couple_id=m.couple_id WHERE m.user_id=$1 AND i.id=$2)`, id.User.ID, input.ImageID).Scan(&ok)
		if err != nil {
			return err
		}
		if !ok {
			return fail(400, "invalid_image")
		}
		err = s.db.QueryRow(r.Context(), `SELECT i.couple_id::text FROM images i WHERE i.id=$1`, input.ImageID).Scan(&couple)
		if err != nil {
			return err
		}
		imageID = &input.ImageID
	}
	if _, err = s.db.Exec(r.Context(), `UPDATE users SET avatar_image_id=$1 WHERE id=$2`, imageID, id.User.ID); err != nil {
		return err
	}
	value := ""
	if imageID != nil {
		value = *imageID
	}
	write(w, 200, map[string]any{"avatar_image_id": value})
	return nil
}