# 伴侣关怀中心 · 设计、数据与验收

本轮交付 `1.0-r64`：首页改版 + 生理期管理 + 情侣关怀 + 服务端与迁移 016。

- 首页保留相恋天数、传达心意、伴侣状态，新增「功能区」和「时光记录」两块。
- 底部导航从五项改为四项：首页 / 地图 / 消息 / 设置；「时光」不再占底部导航，改为首页入口进入。
- 伴侣关怀中心从首页功能区进入，不占底部导航，内部八个页面。
- 健康数据默认只有本人可见；伴侣共享是独立授权，可随时撤销，撤销立即生效。
- 周期预测一律标注为估算，不构成医疗建议。

## 1. 版本与能力标识

| 项目 | 值 |
| --- | --- |
| 版本 | `1.0-r64`（`VERSION` 为唯一来源，`node scripts/set-revision.mjs 64` 同步全部元数据） |
| 数据库迁移 | 新增 `016_cycle_care.sql`，`Migrate` 中 `current < 16` 执行，上限检查改为 `current > 16` |
| 服务端能力标识 | `/account` 的 `features` 增加 `"cycle_care_v1"` |
| 客户端行为 | `Client.snapshot()` 据此设置 `client.care.supported`；不支持时关怀入口仍可见，详情页显示 `careUnsupported` 文案 |

旧客户端连新服务器无影响；新客户端连旧服务器时 `cycle_care_v1` 缺失，所有关怀接口都不调用，避免 404 冒泡成网络错误。

## 2. 数据模型（迁移 016）

所有表都以 `user_id`（个人数据）或 `couple_id`（情侣数据）为归属，个人数据不允许通过客户端传入其他人的 ID 读取。

```sql
CREATE TABLE IF NOT EXISTS cycle_profiles (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  timezone text NOT NULL DEFAULT 'Asia/Shanghai',
  average_cycle_days integer NOT NULL DEFAULT 28 CHECK (average_cycle_days BETWEEN 21 AND 45),
  average_period_days integer NOT NULL DEFAULT 5 CHECK (average_period_days BETWEEN 1 AND 10),
  reminder_enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS period_records (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  start_date date NOT NULL,
  end_date date,
  flow_level text NOT NULL DEFAULT 'medium' CHECK (flow_level IN ('light','medium','heavy')),
  note text NOT NULL DEFAULT '' CHECK (char_length(note) <= 500),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (end_date IS NULL OR end_date >= start_date)
);
CREATE UNIQUE INDEX IF NOT EXISTS period_records_start ON period_records(user_id, start_date);
CREATE INDEX IF NOT EXISTS period_records_user_start ON period_records(user_id, start_date DESC);

CREATE TABLE IF NOT EXISTS cycle_daily_logs (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  log_date date NOT NULL,
  pain_level smallint NOT NULL DEFAULT 0 CHECK (pain_level BETWEEN 0 AND 10),
  mood text NOT NULL DEFAULT '' CHECK (mood IN ('','happy','calm','tired','sad','irritable','anxious')),
  symptoms text[] NOT NULL DEFAULT '{}',
  note text NOT NULL DEFAULT '' CHECK (char_length(note) <= 500),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS cycle_daily_logs_day ON cycle_daily_logs(user_id, log_date);

CREATE TABLE IF NOT EXISTS care_preferences (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  preferred_support text[] NOT NULL DEFAULT '{}',
  avoid_support text[] NOT NULL DEFAULT '{}',
  note text NOT NULL DEFAULT '' CHECK (char_length(note) <= 300),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS care_share_permissions (
  id uuid PRIMARY KEY,
  owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  partner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  couple_id uuid NOT NULL REFERENCES couples(id) ON DELETE CASCADE,
  scope text NOT NULL CHECK (scope IN ('cycle_summary','period_dates','daily_logs','care_preferences')),
  granted_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz,
  CHECK (owner_user_id <> partner_user_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS care_share_active
  ON care_share_permissions(owner_user_id, partner_user_id, scope) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS care_tasks (
  id uuid PRIMARY KEY,
  couple_id uuid NOT NULL REFERENCES couples(id) ON DELETE CASCADE,
  subject_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_by_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
  note text NOT NULL DEFAULT '' CHECK (char_length(note) <= 500),
  due_at timestamptz,
  status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','done')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz
);
CREATE INDEX IF NOT EXISTS care_tasks_couple ON care_tasks(couple_id, status, due_at);

CREATE TABLE IF NOT EXISTS cycle_reminders (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('period_upcoming','period_start','log_nudge','care_task')),
  scheduled_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, kind, scheduled_at)
);
CREATE INDEX IF NOT EXISTS cycle_reminders_pending ON cycle_reminders(user_id, scheduled_at);

CREATE TABLE IF NOT EXISTS cycle_reminder_receipts (
  reminder_id uuid NOT NULL REFERENCES cycle_reminders(id) ON DELETE CASCADE,
  device_id uuid NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (reminder_id, device_id)
);
```

补充约定：

- 经期重叠在事务中检查（插入/修改前查同用户相邻区间），并发创建冲突返回 `409 cycle_overlap`。
- 人均最多 400 条经期记录、500 条每日记录，超限返回 `400 care_limit`；不是静默截断。
- 共享授权只做软撤销（写入 `revoked_at`），保留审计痕迹；撤销后任何旧查询都会因 `revoked_at IS NULL` 条件立即 403/隐藏。
- 解绑（`DELETE /couples/current`）在同一事务里把双方所有有效授权置为撤销，并把 `care_tasks` 保留但双方都不可再访问。

## 3. 隐私与授权模型

| 数据 | 默认可见 | 伴侣可见条件 |
| --- | --- | --- |
| 周期档案 / 预测摘要 | 本人 | `cycle_summary` |
| 经期起止日期 | 本人 | `period_dates` |
| 每日身体记录 | 本人 | `daily_logs` |
| 关怀偏好 | 本人 | `care_preferences` |
| 关怀清单（care_tasks） | 当前情侣双方 | 有效情侣关系即可 |
| 提醒 | 本人 | 不可共享 |

规则：

1. 个人接口的 `user_id` 一律取当前登录身份，忽略任何客户端传入的用户标识。
2. 伴侣数据接口先校验「当前有效情侣关系」（`couples.status='active'` + `couple_members`），再逐项校验授权范围；用 `chatTransaction()` 同一套锁与校验，而不是只比较 `couples.user_a/user_b`。
3. 未授权范围不返回任何数据，也不返回「是否存在记录」这类可推断信息。
4. 解绑后旧授权与旧数据访问全部拒绝（403 `relationship_changed`）。
5. 提醒通知正文只允许「有一条私人提醒」这类固定文案，禁止出现经期日期、症状、情绪、备注。
6. 健康数据不写入聊天缓存、`localStorage` 明文笔记或普通 Notes 表。

## 4. 既有权限缺陷修复

`server/internal/api/media.go` 的 `isMember()` 只比较 `couples.user_a/user_b`，没有检查关系是否仍然有效；而 `unbind()` 会置 `status='ended'` 并删除 `couple_members`。结果：解绑后仍能凭旧 `couple_id` 读共享图片和笔记。

修复：`isMember()` 改为

```sql
SELECT EXISTS(
  SELECT 1 FROM couples c JOIN couple_members m ON m.couple_id = c.id
  WHERE c.id = $1 AND m.user_id = $2 AND c.status = 'active'
)
```

并补充回归测试：解绑后 `GET /notes?couple_id=旧ID`、`GET /images/{旧图}` 均 403，另一对情侣互不可见。

## 5. HTTP API 契约

全部需要 Bearer Token；JSON；错误体 `{"code": "..."}`；状态码沿用现有约定（400 输入、401 未登录、403 无权限、404 不存在、409 冲突）。

### 5.1 个人周期

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/care/profile` | 读取本人档案（不存在时返回默认值，不写库） |
| PUT | `/care/profile` | `{timezone?,average_cycle_days?,average_period_days?,reminder_enabled?}` |
| GET | `/care/periods?from=&to=` | 经期记录，`from/to` 可选，最多 400 条 |
| POST | `/care/periods` | `{id?,start_date,end_date?,flow_level?,note?}`，201 |
| PUT | `/care/periods/{id}` | 同上字段，`id` 必填 |
| DELETE | `/care/periods/{id}` | `{ok:true}` |
| GET | `/care/logs?from=&to=` | 每日记录 |
| POST | `/care/logs` | `{id?,log_date,pain_level?,mood?,symptoms?,note?}`，同一天重复返回 409 |
| PUT | `/care/logs/{id}` | 同上 |
| DELETE | `/care/logs/{id}` | `{ok:true}` |
| GET | `/care/statistics` | 周期长度、平均周期、预测、`estimate:true` |
| GET | `/care/preferences` | `{preferred_support:[],avoid_support:[],note}` |
| PUT | `/care/preferences` | 同上 |

响应示例：

```json
{"profile":{"timezone":"Asia/Shanghai","average_cycle_days":28,"average_period_days":5,"reminder_enabled":true}}
{"periods":[{"id":"...","start_date":"2026-11-01","end_date":"2026-11-05","flow_level":"medium","note":"","created_at":"...","updated_at":"..."}]}
{"statistics":{"average_cycle_days":28,"average_period_days":5,"cycles":[{"start_date":"2026-10-04","end_date":"2026-10-08","length_days":28}],"predicted_start_date":"2026-12-01","predicted_end_date":"2026-12-05","days_until_next":11,"confidence":"estimate"}}
```

### 5.2 情侣共享与伴侣数据

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/care/shares` | `{shares:[{id,scope,granted_at}], scopes:["cycle_summary",...], capabilities:[...]}` |
| POST | `/care/shares` | `{scope}`，201；未绑定 403 `not_bound` |
| DELETE | `/care/shares/{id}` | 只能撤销本人授权，`{ok:true}` |
| GET | `/care/partner` | 只返回已授权范围；返回 `grants:{cycle_summary:bool,...}` 与对应数据 |
| GET | `/care/tasks?status=open` | 当前情侣的关怀清单 |
| POST | `/care/tasks` | `{id?,title,note?,due_at?}`，201 |
| PUT | `/care/tasks/{id}` | `{title?,note?,due_at?,status}` |
| DELETE | `/care/tasks/{id}` | `{ok:true}` |
| GET | `/care/reminders` | 本设备待处理提醒 `{reminders:[{id,kind,scheduled_at}]}` |
| POST | `/care/reminders/receipts` | `{id}`，`{ok:true}` |
| GET | `/care/dashboard` | 首页与关怀首页汇总（见下） |

`/care/partner` 响应示例（只含被授权字段）：

```json
{"partner":{"display_name":"TA","grants":{"cycle_summary":true,"period_dates":false,"daily_logs":false,"care_preferences":true},
 "summary":{"predicted_start_date":"2026-12-01","days_until_next":11,"estimate":true},
 "periods":null,"logs":null,
 "preferences":{"preferred_support":["抱抱"],"avoid_support":["说教"],"note":""}}}
```

`/care/dashboard` 响应：

```json
{"dashboard":{
  "profile":{"average_cycle_days":28,"average_period_days":5,"reminder_enabled":true},
  "today":{"in_period":false,"period_day":0,"has_log":false,"open_tasks":1},
  "prediction":{"start_date":"2026-12-01","days_until":11,"estimate":true},
  "partner":{"display_name":"TA","grants":{"cycle_summary":true},"summary":{...}|null},
  "shares":{"active":["cycle_summary"],"available":["period_dates","daily_logs","care_preferences"]}
}}
```

### 5.3 提醒生成

`RunCareReminders(ctx)` 每 30 秒执行一次（`main.go` 与 `RunAnniversaryReminders` 并列启动）：

1. 只有 `cycle_profiles.reminder_enabled` 的用户参与。
2. 预测下次经期开始日（最近一次 `period_records.start_date` + 平均周期），在开始前 2 天 09:00（个人时区）插入 `period_upcoming`；`UNIQUE(user_id,kind,scheduled_at)` 去重。
3. 连续 3 天没有每日记录且当天已过 20:00 时插入 `log_nudge`（同一天只生成一次）。
4. 清理 24 小时前的提醒与回执，与纪念日提醒一致。

## 6. 前端结构

```
apps/mobile-web/src/
├── core.ts                  # 新类型 + Client 方法 + care 状态 + 能力标识
├── strings.ts               # 全部新增中英文文案（唯一 writer）
├── style.css                # 首页功能区、时光记录、导航样式
├── App.vue                  # 四导航、首页五块、时光子页、关怀中心路由
├── home/
│   ├── HomeCare.vue         # 功能区：伴侣关怀中心入口 + 今日关怀
│   ├── HomeTime.vue         # 我们的时光预览 + 查看全部
│   └── HomeDates.vue        # 共同纪念日预览 + 管理入口
└── care/
    ├── cycle.ts             # 纯函数：日期、预测、统计、校验（可单测）
    ├── CareCenter.vue       # 关怀中心外壳 + 八个页面的导航与返回
    ├── CycleCalendar.vue    # 经期日历（月视图标记 + 记录增删）
    ├── CycleRecord.vue      # 每日记录（疼痛/情绪/症状/备注）
    ├── CycleStatistics.vue  # 周期统计与预测（标注估算）
    ├── CareTasks.vue        # 关怀清单
    ├── CareShareSettings.vue# 共享设置（逐项授权/撤销）
    ├── CareReminders.vue    # 提醒设置
    └── CarePrivacy.vue      # 隐私管理（共享范围说明、全部撤销、删除本人健康数据）
```

页面归属：

- 首页（`tab='home'`）：相恋天数卡片 → 传达心意 → 伴侣状态 → 功能区 → 时光记录。
- 时光子页（`tab='time'`）：`NotesPanel` + `CalendarPanel`，顶部带「返回首页」；只能从首页「查看全部 / 管理」进入，底部导航不再包含「时光」项（该页四项导航都保持未选中，未隐藏，便于随时离开）。
- 关怀中心：`App.vue` 用 `carePage = '' | 'center'` 切换；`CareCenter` 内部维护八个子页面状态；关怀中心打开时隐藏底部导航。
- 通知点击仍只跳到消息或时光，不新增系统入口。

`Client` 新增公开成员：

```ts
care = { supported: false, dashboard: null as CareDashboard | null, profile: null as CareProfile | null,
  periods: [] as PeriodRecord[], logs: [] as CycleLog[], preferences: null as CarePreferences | null,
  shares: [] as CareShare[], partner: null as PartnerCare | null, tasks: [] as CareTask[],
  statistics: null as CareStatistics | null, reminders: [] as CareReminder[] };
```

方法：`loadCareDashboard`, `loadCareProfile`, `saveCareProfile`, `loadPeriods`, `savePeriod`, `deletePeriod`, `loadLogs`, `saveLog`, `deleteLog`, `loadCareStatistics`, `loadCarePreferences`, `saveCarePreferences`, `loadCareShares`, `grantCareShare`, `revokeCareShare`, `loadPartnerCare`, `loadCareTasks`, `saveCareTask`, `deleteCareTask`, `loadCareReminders`, `ackCareReminder`, `clearCare`。所有方法在 `!care.supported` 时抛 `ApiError('upgrade_required')`。

## 7. 文案键名（strings.ts 唯一 writer 维护）

首页：`careCenter` `careCenterHint` `todayCare` `todayCareEmpty` `timeRecords` `viewAll` `commonAnniversaries` `manage` `noMemoriesYet` `careUnsupported` `backHome`

关怀导航：`careHome` `cycleCalendar` `dailyLog` `cycleStats` `careChecklist` `shareSettings` `reminderSettings` `privacy`

周期：`cycleProfile` `averageCycle` `averagePeriod` `reminderEnabled` `periodStart` `periodEnd` `flowLight` `flowMedium` `flowHeavy` `periodNote` `addPeriod` `editPeriod` `deletePeriodConfirm` `cycleOverlap` `careLimit` `painLevel` `mood` `moodHappy` `moodCalm` `moodTired` `moodSad` `moodIrritable` `moodAnxious` `symptoms` `symptomCramps` `symptomHeadache` `symptomFatigue` `symptomBloating` `symptomBackache` `symptomInsomnia` `logToday` `logSaved` `predicted` `estimateOnly` `daysUntilNext` `lastPeriod` `noCycleData` `cycleHistory`

共享：`shareCycleSummary` `sharePeriodDates` `shareDailyLogs` `sharePreferences` `shareHint` `grantShare` `revokeShare` `shareRevoked` `partnerNoShare` `partnerCare` `notBound` `relationshipChanged`

清单/提醒/隐私：`careTaskTitle` `careTaskNote` `careTaskDue` `careTaskDone` `careTaskOpen` `newCareTask` `noCareTasks` `careReminders` `careReminderUpcoming` `careReminderLog` `careReminderTask` `privateReminder` `privacyHint` `revokeAllShares` `deleteHealthData` `deleteHealthDataConfirm` `deleteHealthDataDone`

错误：`care_unsupported` 复用 `upgrade_required`。

## 8. 验收测试

### 8.1 Go 单元测试（无数据库，`go test ./...`）

- 周期统计：2–6 条历史记录的平均周期、平均经期长度、预测日期；无记录返回 `null` 预测，单条记录按档案平均值给出预测（前后端一致）。
- 日期边界：跨月、跨年、2 月 29 日、结束日早于开始日拒绝、未来日期拒绝。
- 校验：周期 21–45、经期 1–10、疼痛 0–10、心情枚举、症状白名单、备注长度。
- 授权辅助函数：scope 白名单、未授权返回 false、撤销后 false。

### 8.2 PostgreSQL 集成测试（`COUPLELINK_TEST_DATABASE_URL`）

- 015 → 016 升级；`Migrate` 重复执行不重复建表、旧聊天/纪念日数据不变。
- 未登录读个人档案 401；读他人记录 403；未授权读伴侣记录 403；授权后 200；撤销后立即 403。
- 解绑后：旧共享授权读取 403、旧 `couple_id` 笔记/图片 403（回归本次修复）。
- 非法日期 400、重复经期 409、非法周期长度 400、并发创建不产生重叠记录。
- 两台设备不重复接收同一提醒（回执去重）。

### 8.3 前端测试（`npm --prefix apps/mobile-web test`）

- `care/cycle.ts` 纯函数：月视图生成、经期区间标记、预测与统计与 Go 端一致、日期校验。
- 现有 `core.test.ts`、`memories.test.ts`、`reception.test.ts` 必须继续通过。

### 8.4 浏览器验收

`vite build` 后用本地静态服务或 `npm --prefix apps/mobile-web run dev` 在 320 / 360 / 390 / 桌面宽度检查：四导航、首页五块顺序、时光子页返回、关怀八个页面、经期日历横向不溢出、表单可保存、三主题与中英文。

### 8.5 交付前检查

```
npm run check
npm --prefix apps/mobile-web test
npm --prefix apps/mobile-web run build
cd server && gofmt -l . && go vet ./... && go test ./...
COUPLELINK_TEST_DATABASE_URL=... go test ./internal/api -run TestPostgres -count=1
```

## 9. 开发顺序

1. P0 权限修复：`media.go` 有效关系校验 + 回归测试。
2. P1 首页改版：四导航、时光整合、功能区入口；保留原聊天/纪念日/笔记能力。
3. P2 生理期：迁移 016、个人周期 API、日历/记录/统计页面。
4. P3 情侣关怀：共享授权、伴侣数据、关怀清单、提醒、隐私管理。
5. P4 提醒与统计打磨：`RunCareReminders`、通知文案隐私、浏览器多宽度验收。

## 10. 边界

- 预测是估算，不代替医疗判断；未实现任何 AI 建议。
- 不修改用户手机、不连接真机；系统通知与振动效果仍需真机验收。
- 非商业源码许可证：保留署名与 LICENSE，商业使用需原作者书面许可。
