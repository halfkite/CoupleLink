package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	backupFormat      = "couplelink-backup"
	backupGroupFormat = "couplelink-group"
	backupVersion     = 1
)

type backupFile struct {
	Format     string        `json:"format"`
	Version    int           `json:"version"`
	AppVersion string        `json:"app_version"`
	ExportedAt string        `json:"exported_at"`
	Groups     []backupGroup `json:"groups"`
}

type backupGroupFile struct {
	Format     string      `json:"format"`
	Version    int         `json:"version"`
	AppVersion string      `json:"app_version"`
	ExportedAt string      `json:"exported_at"`
	Group      backupGroup `json:"group"`
}

type backupGroup struct {
	CoupleID  string          `json:"couple_id"`
	Key       string          `json:"key"`
	Label     string          `json:"label"`
	StartAt   string          `json:"start_at"`
	Timezone  string          `json:"timezone"`
	Status    string          `json:"status"`
	CreatedAt string          `json:"created_at"`
	RemarkA   string          `json:"remark_a"`
	RemarkB   string          `json:"remark_b"`
	Archived  bool            `json:"archived,omitempty"`
	Members   []backupMember  `json:"members"`
	Messages  []backupMessage `json:"messages"`
}

type backupMember struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Gender      string `json:"gender"`
}

type backupMessage struct {
	ID        string `json:"id"`
	SenderID  string `json:"sender_id"`
	Body      string `json:"body"`
	Seq       int64  `json:"seq"`
	CreatedAt string `json:"created_at"`
}

func (s *Server) adminExport(w http.ResponseWriter, r *http.Request) error {
	if _, ok := s.requireAdmin(w, r, false); !ok {
		return nil
	}
	file, err := s.collectBackup(r.Context())
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	name := "couplelink-backup-" + time.Now().Format("20060102-150405") + ".json"
	return writeDownload(w, name, payload)
}

// adminGroupExport downloads one group (info, key, remarks and its chat) so a
// single conversation can be kept or moved without the other groups.
func (s *Server) adminGroupExport(w http.ResponseWriter, r *http.Request) error {
	if _, ok := s.requireAdmin(w, r, false); !ok {
		return nil
	}
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		return fail(400, "invalid_input")
	}
	group, err := s.loadGroup(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		redirectAdminError(w, r, "组不存在")
		return nil
	}
	if err != nil {
		return err
	}
	file := backupGroupFile{Format: backupGroupFormat, Version: backupVersion, AppVersion: s.cfg.Version,
		ExportedAt: time.Now().UTC().Format(time.RFC3339), Group: *group}
	payload, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	name := "couplelink-group-" + strings16(id) + "-" + time.Now().Format("20060102-150405") + ".json"
	return writeDownload(w, name, payload)
}

func writeDownload(w http.ResponseWriter, name string, payload []byte) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, err := w.Write(append(payload, '\n'))
	return err
}

func strings16(id string) string {
	clean := ""
	for _, r := range id {
		if r == '-' {
			continue
		}
		clean += string(r)
	}
	if len(clean) > 8 {
		clean = clean[:8]
	}
	return clean
}

func (s *Server) collectBackup(ctx context.Context) (*backupFile, error) {
	file := &backupFile{
		Format:     backupFormat,
		Version:    backupVersion,
		AppVersion: s.cfg.Version,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
	}
	rows, err := s.db.Query(ctx, `SELECT id::text FROM couples ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		group, err := s.loadGroup(ctx, id)
		if err != nil {
			return nil, err
		}
		file.Groups = append(file.Groups, *group)
	}
	if file.Groups == nil {
		file.Groups = []backupGroup{}
	}
	return file, nil
}

// loadGroup reads one couple with both members and every message.
func (s *Server) loadGroup(ctx context.Context, coupleID string) (*backupGroup, error) {
	var group backupGroup
	var userA, userB string
	err := s.db.QueryRow(ctx, `SELECT c.id::text,
  COALESCE((SELECT k.key_plain FROM access_keys k WHERE k.couple_id=c.id ORDER BY k.created_at DESC LIMIT 1),''),
  COALESCE((SELECT k.label FROM access_keys k WHERE k.couple_id=c.id ORDER BY k.created_at DESC LIMIT 1),''),
  c.start_at::text,c.timezone,c.status,to_char(c.created_at,'YYYY-MM-DD"T"HH24:MI:SSOF'),c.remark_a,c.remark_b,
  c.user_a::text,c.user_b::text,(c.archived_at IS NOT NULL)
  FROM couples c WHERE c.id=$1`, coupleID).Scan(&group.CoupleID, &group.Key, &group.Label, &group.StartAt,
		&group.Timezone, &group.Status, &group.CreatedAt, &group.RemarkA, &group.RemarkB, &userA, &userB, &group.Archived)
	if err != nil {
		return nil, err
	}
	memberRows, err := s.db.Query(ctx, `SELECT id::text,username,display_name,gender FROM users WHERE id IN ($1,$2)`, userA, userB)
	if err != nil {
		return nil, err
	}
	members := map[string]backupMember{}
	for memberRows.Next() {
		var member backupMember
		if err := memberRows.Scan(&member.ID, &member.Username, &member.DisplayName, &member.Gender); err != nil {
			memberRows.Close()
			return nil, err
		}
		members[member.ID] = member
	}
	memberRows.Close()
	if err := memberRows.Err(); err != nil {
		return nil, err
	}
	group.Members = []backupMember{members[userA], members[userB]}
	messageRows, err := s.db.Query(ctx, `SELECT id::text,sender_id::text,body,seq,to_char(created_at,'YYYY-MM-DD"T"HH24:MI:SSOF')
  FROM messages WHERE couple_id=$1 ORDER BY seq`, coupleID)
	if err != nil {
		return nil, err
	}
	for messageRows.Next() {
		var message backupMessage
		if err := messageRows.Scan(&message.ID, &message.SenderID, &message.Body, &message.Seq, &message.CreatedAt); err != nil {
			messageRows.Close()
			return nil, err
		}
		group.Messages = append(group.Messages, message)
	}
	messageRows.Close()
	if err := messageRows.Err(); err != nil {
		return nil, err
	}
	if group.Messages == nil {
		group.Messages = []backupMessage{}
	}
	return &group, nil
}

func (s *Server) adminImport(w http.ResponseWriter, r *http.Request) error {
	if _, ok := s.requireAdmin(w, r, false); !ok {
		return nil
	}
	mode := ""
	raw, ok := readUpload(w, r, &mode)
	if !ok {
		return nil
	}
	if mode != "merge" && mode != "overwrite" {
		redirectAdminError(w, r, "请选择覆盖或合并")
		return nil
	}
	var backup backupFile
	if err := json.Unmarshal(raw, &backup); err != nil {
		redirectAdminError(w, r, "不是有效的 JSON 备份")
		return nil
	}
	if backup.Format == backupGroupFormat {
		redirectAdminError(w, r, "这是单个组的备份，请在对应组上使用“导入聊天”")
		return nil
	}
	if err := validateBackup(&backup); err != nil {
		redirectAdminError(w, r, err.Error())
		return nil
	}
	added, messages, skipped, err := s.importBackup(r.Context(), &backup, mode)
	if err != nil {
		return err
	}
	http.Redirect(w, r, "/admin?imported="+url.QueryEscape(fmt.Sprintf("新增 %d 组、%d 条聊天，跳过 %d 组", added, messages, skipped)), http.StatusSeeOther)
	return nil
}

// adminGroupImport merges a single-group backup into the selected group. The two
// members are mapped by position, so a chat exported from another group still
// lands on the right side.
func (s *Server) adminGroupImport(w http.ResponseWriter, r *http.Request) error {
	if _, ok := s.requireAdmin(w, r, false); !ok {
		return nil
	}
	target := r.PathValue("id")
	if !uuidPattern.MatchString(target) {
		return fail(400, "invalid_input")
	}
	mode := ""
	raw, ok := readUpload(w, r, &mode)
	if !ok {
		return nil
	}
	group, err := parseGroupBackup(raw)
	if err != nil {
		redirectAdminError(w, r, err.Error())
		return nil
	}
	if err := validateGroup(*group); err != nil {
		redirectAdminError(w, r, err.Error())
		return nil
	}
	var userA, userB string
	var archived bool
	err = s.db.QueryRow(r.Context(), `SELECT user_a::text,user_b::text,(archived_at IS NOT NULL) FROM couples WHERE id=$1`, target).Scan(&userA, &userB, &archived)
	if errors.Is(err, pgx.ErrNoRows) {
		redirectAdminError(w, r, "组不存在")
		return nil
	}
	if err != nil {
		return err
	}
	if archived {
		redirectAdminError(w, r, "回收站里的组，先恢复再导入")
		return nil
	}
	imported, err := s.importGroupMessages(r.Context(), target, *group, userA, userB)
	if err != nil {
		return err
	}
	http.Redirect(w, r, "/admin?imported="+url.QueryEscape(fmt.Sprintf("该组新增 %d 条聊天", imported)), http.StatusSeeOther)
	return nil
}

func redirectAdminError(w http.ResponseWriter, r *http.Request, message string) {
	http.Redirect(w, r, "/admin?error="+url.QueryEscape(message), http.StatusSeeOther)
}
func readUpload(w http.ResponseWriter, r *http.Request, mode *string) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		redirectAdminError(w, r, "文件过大或表单无效")
		return nil, false
	}
	*mode = r.FormValue("mode")
	file, _, err := r.FormFile("file")
	if err != nil {
		redirectAdminError(w, r, "请选择备份文件")
		return nil, false
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 64<<20))
	if err != nil {
		redirectAdminError(w, r, "读取文件失败")
		return nil, false
	}
	return raw, true
}

func parseGroupBackup(raw []byte) (*backupGroup, error) {
	var head struct {
		Format  string `json:"format"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, errors.New("不是有效的 JSON 备份")
	}
	switch head.Format {
	case backupGroupFormat:
		var file backupGroupFile
		if err := json.Unmarshal(raw, &file); err != nil {
			return nil, errors.New("不是有效的组备份")
		}
		if file.Version <= 0 || file.Version > backupVersion {
			return nil, fmt.Errorf("组备份版本 %d 不受支持", file.Version)
		}
		return &file.Group, nil
	case backupFormat:
		var file backupFile
		if err := json.Unmarshal(raw, &file); err != nil {
			return nil, errors.New("不是有效的备份")
		}
		if len(file.Groups) != 1 {
			return nil, errors.New("整库备份请用上面的“导入备份”")
		}
		if file.Version <= 0 || file.Version > backupVersion {
			return nil, fmt.Errorf("备份版本 %d 不受支持", file.Version)
		}
		return &file.Groups[0], nil
	default:
		return nil, errors.New("不是 CoupleLink 备份文件")
	}
}

func validateGroup(group backupGroup) error {
	if !uuidPattern.MatchString(group.CoupleID) {
		return errors.New("组编号无效")
	}
	if len(group.Members) != 2 {
		return errors.New("组备份必须有两位成员")
	}
	seen := map[string]bool{}
	for _, member := range group.Members {
		if !uuidPattern.MatchString(member.ID) || !validText(member.Username, 64) || !validText(member.DisplayName, 40) || !validGender(member.Gender) {
			return errors.New("成员信息无效")
		}
		seen[member.ID] = true
	}
	if group.Key != "" && !accessKeyPattern.Match(group.Key) {
		return errors.New("密钥格式无效")
	}
	if _, err := time.Parse(time.DateOnly, group.StartAt); err != nil {
		return errors.New("恋爱起点无效")
	}
	if group.Timezone == "" {
		return errors.New("缺少时区")
	}
	if _, err := time.LoadLocation(group.Timezone); err != nil {
		return errors.New("时区无效")
	}
	if len(group.Messages) > 200000 {
		return errors.New("聊天记录过多")
	}
	for _, message := range group.Messages {
		if !uuidPattern.MatchString(message.ID) || !seen[message.SenderID] || message.Seq <= 0 || len([]rune(message.Body)) < 1 || len([]rune(message.Body)) > 4000 {
			return errors.New("存在无效的聊天记录")
		}
	}
	return nil
}

func validateBackup(backup *backupFile) error {
	if backup.Format != backupFormat {
		return errors.New("不是 CoupleLink 备份文件")
	}
	if backup.Version <= 0 {
		return errors.New("备份缺少版本号")
	}
	if backup.Version > backupVersion {
		return fmt.Errorf("备份版本 %d 高于本程序支持的 %d，请先升级服务端", backup.Version, backupVersion)
	}
	if len(backup.Groups) > 1000 {
		return errors.New("备份包含的组过多")
	}
	for index, group := range backup.Groups {
		if err := validateGroup(group); err != nil {
			return fmt.Errorf("第 %d 组：%s", index+1, err.Error())
		}
	}
	return nil
}

var backupTables = []string{
	"message_receipts", "messages", "interactions", "anniversary_receipts", "anniversary_reminders", "anniversaries",
	"sync_events", "sync_heads", "realtime_outbox", "sessions", "devices", "invitations",
	"couple_members", "couples", "access_keys", "users",
}

func (s *Server) importBackup(ctx context.Context, backup *backupFile, mode string) (int, int, int, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	defer tx.Rollback(ctx)
	if mode == "overwrite" {
		if _, err = tx.Exec(ctx, `TRUNCATE `+joinTableNames()+` RESTART IDENTITY`); err != nil {
			return 0, 0, 0, err
		}
	}
	added, messages, skipped := 0, 0, 0
	for _, group := range backup.Groups {
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM couples WHERE id=$1)`, group.CoupleID).Scan(&exists); err != nil {
			return 0, 0, 0, err
		}
		if !exists {
			if err = insertBackupGroup(ctx, tx, group); err != nil {
				return 0, 0, 0, err
			}
			added++
		} else if mode == "overwrite" {
			return 0, 0, 0, errors.New("覆盖导入时组仍然存在")
		} else {
			skipped++
		}
		for _, message := range group.Messages {
			tag, err := tx.Exec(ctx, `INSERT INTO messages(id,couple_id,sender_id,body,seq,created_at)
  VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, message.ID, group.CoupleID, message.SenderID, message.Body, message.Seq, message.CreatedAt)
			if err != nil {
				return 0, 0, 0, err
			}
			messages += int(tag.RowsAffected())
		}
		if _, err = tx.Exec(ctx, `UPDATE couples SET message_seq=GREATEST(message_seq,COALESCE((SELECT max(seq) FROM messages WHERE couple_id=$1),0)) WHERE id=$1`, group.CoupleID); err != nil {
			return 0, 0, 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, 0, 0, err
	}
	return added, messages, skipped, nil
}

// importGroupMessages adds the messages of one exported group to an existing
// group, mapping senders by their position in the member list.
func (s *Server) importGroupMessages(ctx context.Context, coupleID string, group backupGroup, userA, userB string) (int, error) {
	mapping := map[string]string{}
	for index, member := range group.Members {
		switch member.ID {
		case userA:
			mapping[member.ID] = userA
		case userB:
			mapping[member.ID] = userB
		default:
			if index == 0 {
				mapping[member.ID] = userA
			} else {
				mapping[member.ID] = userB
			}
		}
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var next int64
	if err = tx.QueryRow(ctx, `SELECT message_seq FROM couples WHERE id=$1 FOR UPDATE`, coupleID).Scan(&next); err != nil {
		return 0, err
	}
	imported := 0
	for _, message := range group.Messages {
		sender, ok := mapping[message.SenderID]
		if !ok {
			continue
		}
		var seen bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages
  WHERE couple_id=$1 AND sender_id=$2 AND body=$3 AND created_at=$4)`,
			coupleID, sender, message.Body, message.CreatedAt).Scan(&seen); err != nil {
			return 0, err
		}
		if seen {
			continue
		}
		id, err := identifier()
		if err != nil {
			return 0, err
		}
		next++
		if _, err = tx.Exec(ctx, `INSERT INTO messages(id,couple_id,sender_id,body,seq,created_at)
  VALUES($1,$2,$3,$4,$5,$6)`, id, coupleID, sender, message.Body, next, message.CreatedAt); err != nil {
			return 0, err
		}
		imported++
	}
	if _, err = tx.Exec(ctx, `UPDATE couples SET message_seq=$1 WHERE id=$2`, next, coupleID); err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return imported, nil
}
func joinTableNames() string {
	out := ""
	for index, name := range backupTables {
		if index > 0 {
			out += ","
		}
		out += name
	}
	return out
}

func insertBackupGroup(ctx context.Context, tx pgx.Tx, group backupGroup) error {
	unusable, err := randomToken()
	if err != nil {
		return err
	}
	for _, member := range group.Members {
		if _, err = tx.Exec(ctx, `INSERT INTO users(id,username,display_name,password_hash,gender)
  VALUES($1,$2,$3,$4,$5)`, member.ID, member.Username, member.DisplayName, tokenHash(unusable), member.Gender); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO sync_heads(user_id) VALUES($1) ON CONFLICT DO NOTHING`, member.ID); err != nil {
			return err
		}
	}
	status := group.Status
	if status != "active" && status != "ended" {
		status = "active"
	}
	if _, err = tx.Exec(ctx, `INSERT INTO couples(id,user_a,user_b,start_at,timezone,status,remark_a,remark_b,archived_at)
  VALUES($1,$2,$3,$4,$5,$6,$7,$8,CASE WHEN $9 THEN now() ELSE NULL END)`, group.CoupleID, group.Members[0].ID, group.Members[1].ID, group.StartAt, group.Timezone, status, group.RemarkA, group.RemarkB, group.Archived); err != nil {
		return err
	}
	for _, member := range group.Members {
		if _, err = tx.Exec(ctx, `INSERT INTO couple_members(user_id,couple_id) VALUES($1,$2) ON CONFLICT (user_id) DO NOTHING`, member.ID, group.CoupleID); err != nil {
			return err
		}
	}
	if group.Key != "" {
		keyID, err := identifier()
		if err != nil {
			return err
		}
		hint := group.Key
		if len(hint) > 10 {
			hint = hint[:10]
		}
		if _, err = tx.Exec(ctx, `INSERT INTO access_keys(id,key_hash,key_plain,key_hint,couple_id,label,revoked_at)
  VALUES($1,$2,$3,$4,$5,$6,CASE WHEN $7 THEN now() ELSE NULL END) ON CONFLICT DO NOTHING`,
			keyID, tokenHash(group.Key), group.Key, hint, group.CoupleID, group.Label, group.Archived); err != nil {
			return err
		}
	}
	return nil
}