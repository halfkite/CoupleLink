package migrations

import _ "embed"

//go:embed 001_accounts.sql
var Accounts string

//go:embed 002_chat.sql
var Chat string

//go:embed 003_interactions.sql
var Interactions string

//go:embed 004_memories.sql
var Memories string

//go:embed 005_access_keys.sql
var AccessKeys string

//go:embed 006_admin.sql
var Admin string

//go:embed 007_admin_plain.sql
var AdminPlain string

//go:embed 008_group_archive.sql
var GroupArchive string

//go:embed 009_read_cursors.sql
var ReadCursors string

//go:embed 010_message_recall.sql
var MessageRecall string

//go:embed 011_media_notes.sql
var MediaNotes string

//go:embed 012_message_photo_body.sql
var MessagePhotoBody string

//go:embed 013_key_slots.sql
var KeySlots string

//go:embed 014_user_avatar.sql
var UserAvatar string

//go:embed 015_note_featured.sql
var NoteFeatured string

//go:embed 016_cycle_care.sql
var CycleCare string
