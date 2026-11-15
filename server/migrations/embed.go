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