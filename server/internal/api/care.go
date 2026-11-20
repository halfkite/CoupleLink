package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Bounds shared by the HTTP handlers, the reminder worker and the unit tests.
const (
	careMaxPeriods        = 400
	careMaxLogs           = 500
	careMaxTasks          = 200
	careMaxNoteRunes      = 500
	careMaxPreferenceNote = 300
	careMaxTitleRunes     = 120
	careMaxSymptoms       = 6
	careMaxTags           = 12
	careMaxTagRunes       = 24
	careMinCycleDays      = 21
	careMaxCycleDays      = 45
	careMinPeriodDays     = 1
	careMaxPeriodDays     = 10
	careReminderLeadDays  = 2
	careReminderHour      = 9
	careLogNudgeHour      = 20
	careReminderWindow    = 24 * time.Hour
	careStatisticsConf    = "estimate"
)

// careDefaultProfile is returned without a database row so the client always
// has a usable profile.
var careDefaultProfile = careProfile{Timezone: "Asia/Shanghai", AverageCycleDays: 28, AveragePeriodDays: 5, ReminderEnabled: true}

var careScopeList = []string{"cycle_summary", "period_dates", "daily_logs", "care_preferences"}
var careFlowLevels = []string{"light", "medium", "heavy"}
var careMoodLevels = []string{"", "happy", "calm", "tired", "sad", "irritable", "anxious"}
var careSymptomList = []string{"cramps", "headache", "fatigue", "bloating", "backache", "insomnia"}

type careProfile struct {
	Timezone          string `json:"timezone"`
	AverageCycleDays  int    `json:"average_cycle_days"`
	AveragePeriodDays int    `json:"average_period_days"`
	ReminderEnabled   bool   `json:"reminder_enabled"`
}

type carePeriodRecord struct {
	ID        string    `json:"id"`
	StartDate string    `json:"start_date"`
	EndDate   *string   `json:"end_date"`
	FlowLevel string    `json:"flow_level"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const carePeriodColumns = `id::text,start_date::text,end_date::text,flow_level,note,created_at,updated_at`

func scanCarePeriod(row pgx.Row) (carePeriodRecord, error) {
	var record carePeriodRecord
	err := row.Scan(&record.ID, &record.StartDate, &record.EndDate, &record.FlowLevel, &record.Note, &record.CreatedAt, &record.UpdatedAt)
	return record, err
}

type careLog struct {
	ID        string    `json:"id"`
	LogDate   string    `json:"log_date"`
	PainLevel int       `json:"pain_level"`
	Mood      string    `json:"mood"`
	Symptoms  []string  `json:"symptoms"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const careLogColumns = `id::text,log_date::text,pain_level,mood,symptoms,note,created_at,updated_at`

func scanCareLog(row pgx.Row) (careLog, error) {
	var record careLog
	err := row.Scan(&record.ID, &record.LogDate, &record.PainLevel, &record.Mood, &record.Symptoms, &record.Note, &record.CreatedAt, &record.UpdatedAt)
	if record.Symptoms == nil {
		record.Symptoms = []string{}
	}
	return record, err
}

type carePreferences struct {
	PreferredSupport []string `json:"preferred_support"`
	AvoidSupport     []string `json:"avoid_support"`
	Note             string   `json:"note"`
}

type careShare struct {
	ID        string    `json:"id"`
	Scope     string    `json:"scope"`
	GrantedAt time.Time `json:"granted_at"`
}

// careTask is the checklist row as the client sees it: the two membership flags
// are computed for the caller so the client never needs raw user identifiers.
type careTask struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Note            string     `json:"note"`
	DueAt           *time.Time `json:"due_at"`
	Status          string     `json:"status"`
	CreatedByMe     bool       `json:"created_by_me"`
	SubjectMe       bool       `json:"subject_me"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	CompletedAt     *time.Time `json:"completed_at"`
	SubjectUserID   string     `json:"-"`
	CreatedByUserID string     `json:"-"`
}

const careTaskColumns = `id::text,subject_user_id::text,created_by_user_id::text,title,note,due_at,status,created_at,updated_at,completed_at`

func scanCareTask(row pgx.Row) (careTask, error) {
	var record careTask
	err := row.Scan(&record.ID, &record.SubjectUserID, &record.CreatedByUserID, &record.Title, &record.Note, &record.DueAt, &record.Status, &record.CreatedAt, &record.UpdatedAt, &record.CompletedAt)
	return record, err
}

func careTaskFor(record careTask, viewer string) careTask {
	record.CreatedByMe = record.CreatedByUserID == viewer
	record.SubjectMe = record.SubjectUserID == viewer
	return record
}

// careCycle is one history row: length_days is the distance to the next start,
// so the newest row always reports null.
type careCycle struct {
	StartDate  string  `json:"start_date"`
	EndDate    *string `json:"end_date"`
	LengthDays *int    `json:"length_days"`
}

type careStatistics struct {
	AverageCycleDays   int         `json:"average_cycle_days"`
	AveragePeriodDays  int         `json:"average_period_days"`
	Cycles             []careCycle `json:"cycles"`
	PredictedStartDate *string     `json:"predicted_start_date"`
	PredictedEndDate   *string     `json:"predicted_end_date"`
	DaysUntilNext      *int        `json:"days_until_next"`
	Confidence         string      `json:"confidence"`
}

// careCycleRecord is the database-free shape the statistics helper works on.
type careCycleRecord struct {
	StartDate string
	EndDate   string
}

type carePartnerSummary struct {
	PredictedStartDate *string `json:"predicted_start_date"`
	DaysUntilNext      *int    `json:"days_until_next"`
	Estimate           bool    `json:"estimate"`
}

type carePartnerPeriod struct {
	ID        string  `json:"id"`
	StartDate string  `json:"start_date"`
	EndDate   *string `json:"end_date"`
}

type carePartnerView struct {
	DisplayName string              `json:"display_name"`
	Grants      map[string]bool     `json:"grants"`
	Summary     *carePartnerSummary `json:"summary"`
	Periods     []carePartnerPeriod `json:"periods"`
	Logs        []careLog           `json:"logs"`
	Preferences *carePreferences    `json:"preferences"`
}

type careDashboardToday struct {
	InPeriod  bool `json:"in_period"`
	PeriodDay int  `json:"period_day"`
	HasLog    bool `json:"has_log"`
	OpenTasks int  `json:"open_tasks"`
}

type careDashboardPrediction struct {
	StartDate *string `json:"start_date"`
	DaysUntil *int    `json:"days_until"`
	Estimate  bool    `json:"estimate"`
}

type careDashboardPartner struct {
	DisplayName string              `json:"display_name"`
	Grants      map[string]bool     `json:"grants"`
	Summary     *carePartnerSummary `json:"summary"`
}

type careDashboardShares struct {
	Active    []string `json:"active"`
	Available []string `json:"available"`
}

type careDashboard struct {
	Profile    careProfile             `json:"profile"`
	Today      careDashboardToday      `json:"today"`
	Prediction careDashboardPrediction `json:"prediction"`
	Partner    *careDashboardPartner   `json:"partner"`
	Shares     careDashboardShares     `json:"shares"`
}

// careQuerier lets the read helpers serve both a pooled connection and a
// request transaction.
type careQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func careContains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func careScopeValid(scope string) bool { return careContains(careScopeList, scope) }

// activeCareScopes answers every known scope explicitly: clients render a fixed
// four-row permission table, so a missing key would be ambiguous while "false"
// states that the scope is known but not granted.
func activeCareScopes(scopes []string) map[string]bool {
	active := make(map[string]bool, len(careScopeList))
	for _, scope := range careScopeList {
		active[scope] = false
	}
	for _, scope := range scopes {
		if careScopeValid(scope) {
			active[scope] = true
		}
	}
	return active
}

func careGranted(active map[string]bool, scope string) bool { return active[scope] }

// validCareDate accepts canonical calendar dates only; time.Parse rejects
// impossible days such as February 30.
func validCareDate(value string) bool {
	parsed, err := time.Parse(time.DateOnly, value)
	return err == nil && parsed.Format(time.DateOnly) == value && parsed.Year() >= 1
}

// validCareText accepts free text up to max runes without control characters.
func validCareText(value string, max int) bool {
	return validOptionalText(value) && utf8.RuneCountInString(value) <= max
}

func normalizeCareSymptoms(values []string) ([]string, bool) {
	symptoms := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		if !careContains(careSymptomList, value) {
			return nil, false
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		symptoms = append(symptoms, value)
	}
	if len(symptoms) > careMaxSymptoms {
		return nil, false
	}
	return symptoms, true
}

func normalizeCareTags(values []string) ([]string, bool) {
	tags := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		tag := strings.TrimSpace(value)
		if tag == "" || !validCareText(tag, careMaxTagRunes) {
			return nil, false
		}
		if seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
		if len(tags) > careMaxTags {
			return nil, false
		}
	}
	return tags, true
}

// careRangesOverlap reports whether two inclusive date ranges share a day; an
// empty end covers only the start day.
func careRangesOverlap(aStart, aEnd, bStart, bEnd string) (bool, error) {
	aFirst, err := time.Parse(time.DateOnly, aStart)
	if err != nil {
		return false, err
	}
	aLast := aFirst
	if aEnd != "" {
		if aLast, err = time.Parse(time.DateOnly, aEnd); err != nil {
			return false, err
		}
	}
	bFirst, err := time.Parse(time.DateOnly, bStart)
	if err != nil {
		return false, err
	}
	bLast := bFirst
	if bEnd != "" {
		if bLast, err = time.Parse(time.DateOnly, bEnd); err != nil {
			return false, err
		}
	}
	return !aLast.Before(bFirst) && !bLast.Before(aFirst), nil
}

func careOverlapsExisting(records []careCycleRecord, start, end string) (bool, error) {
	for _, record := range records {
		overlap, err := careRangesOverlap(record.StartDate, record.EndDate, start, end)
		if err != nil {
			return false, err
		}
		if overlap {
			return true, nil
		}
	}
	return false, nil
}

func careCycleRecords(periods []carePeriodRecord) []careCycleRecord {
	records := make([]careCycleRecord, 0, len(periods))
	for _, record := range periods {
		value := careCycleRecord{StartDate: record.StartDate}
		if record.EndDate != nil {
			value.EndDate = *record.EndDate
		}
		records = append(records, value)
	}
	return records
}

// careRound is half-up integer rounding; the inputs are always non-negative.
func careRound(sum, count int) int { return (sum + count/2) / count }

// computeCareStatistics derives the shared estimate. Only gaps between 15 and 60
// days feed the average cycle so a skipped month or an irregular log cannot
// distort the prediction; the profile values cover the remaining cases.
func computeCareStatistics(records []careCycleRecord, fallbackCycle, fallbackPeriod int, today time.Time) (careStatistics, error) {
	statistics := careStatistics{
		AverageCycleDays:  fallbackCycle,
		AveragePeriodDays: fallbackPeriod,
		Cycles:            []careCycle{},
		Confidence:        careStatisticsConf,
	}
	sorted := append([]careCycleRecord(nil), records...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].StartDate < sorted[j].StartDate })
	starts := make([]time.Time, len(sorted))
	ends := make([]*time.Time, len(sorted))
	for index, record := range sorted {
		start, err := time.Parse(time.DateOnly, record.StartDate)
		if err != nil {
			return careStatistics{}, err
		}
		starts[index] = start
		if record.EndDate != "" {
			end, err := time.Parse(time.DateOnly, record.EndDate)
			if err != nil {
				return careStatistics{}, err
			}
			ends[index] = &end
		}
	}
	cycleSum, cycleCount, periodSum, periodCount := 0, 0, 0, 0
	for index := range sorted {
		var endPtr *string
		if ends[index] != nil {
			value := sorted[index].EndDate
			endPtr = &value
			periodSum += int(ends[index].Sub(starts[index]).Hours()/24) + 1
			periodCount++
		}
		cycle := careCycle{StartDate: sorted[index].StartDate, EndDate: endPtr}
		if index+1 < len(starts) {
			gap := int(starts[index+1].Sub(starts[index]).Hours() / 24)
			cycle.LengthDays = &gap
			if gap >= 15 && gap <= 60 {
				cycleSum += gap
				cycleCount++
			}
		}
		statistics.Cycles = append(statistics.Cycles, cycle)
	}
	if cycleCount > 0 {
		statistics.AverageCycleDays = careRound(cycleSum, cycleCount)
	}
	if periodCount > 0 {
		statistics.AveragePeriodDays = careRound(periodSum, periodCount)
	}
	if len(starts) > 0 {
		predicted := starts[len(starts)-1].AddDate(0, 0, statistics.AverageCycleDays)
		predictedEnd := predicted.AddDate(0, 0, statistics.AveragePeriodDays-1)
		startText := predicted.Format(time.DateOnly)
		endText := predictedEnd.Format(time.DateOnly)
		localToday := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
		days := int(predicted.Sub(localToday).Hours() / 24)
		statistics.PredictedStartDate = &startText
		statistics.PredictedEndDate = &endText
		statistics.DaysUntilNext = &days
	}
	return statistics, nil
}

func scanCareProfile(ctx context.Context, q careQuerier, user string) (careProfile, error) {
	profile := careDefaultProfile
	err := q.QueryRow(ctx, `SELECT timezone,average_cycle_days,average_period_days,reminder_enabled FROM cycle_profiles WHERE user_id=$1`, user).
		Scan(&profile.Timezone, &profile.AverageCycleDays, &profile.AveragePeriodDays, &profile.ReminderEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return careDefaultProfile, nil
	}
	if err != nil {
		return careProfile{}, err
	}
	return profile, nil
}

func scanCarePreferences(ctx context.Context, q careQuerier, user string) (carePreferences, error) {
	value := carePreferences{PreferredSupport: []string{}, AvoidSupport: []string{}}
	err := q.QueryRow(ctx, `SELECT preferred_support,avoid_support,note FROM care_preferences WHERE user_id=$1`, user).
		Scan(&value.PreferredSupport, &value.AvoidSupport, &value.Note)
	if errors.Is(err, pgx.ErrNoRows) {
		return carePreferences{PreferredSupport: []string{}, AvoidSupport: []string{}}, nil
	}
	if err != nil {
		return carePreferences{}, err
	}
	if value.PreferredSupport == nil {
		value.PreferredSupport = []string{}
	}
	if value.AvoidSupport == nil {
		value.AvoidSupport = []string{}
	}
	return value, nil
}

func listCarePeriods(ctx context.Context, q careQuerier, user, from, to string) ([]carePeriodRecord, error) {
	rows, err := q.Query(ctx, `SELECT `+carePeriodColumns+` FROM period_records WHERE user_id=$1
  AND (NULLIF($2,'')::date IS NULL OR start_date>=NULLIF($2,'')::date)
  AND (NULLIF($3,'')::date IS NULL OR start_date<=NULLIF($3,'')::date)
	 ORDER BY start_date,id LIMIT `+strconv.Itoa(careMaxPeriods), user, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []carePeriodRecord{}
	for rows.Next() {
		record, scanErr := scanCarePeriod(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, record)
	}
	return items, rows.Err()
}

func listCareLogs(ctx context.Context, q careQuerier, user, from, to string, limit int) ([]careLog, error) {
	rows, err := q.Query(ctx, `SELECT `+careLogColumns+` FROM cycle_daily_logs WHERE user_id=$1
  AND (NULLIF($2,'')::date IS NULL OR log_date>=NULLIF($2,'')::date)
  AND (NULLIF($3,'')::date IS NULL OR log_date<=NULLIF($3,'')::date)
  ORDER BY log_date DESC,id LIMIT $4`, user, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []careLog{}
	for rows.Next() {
		record, scanErr := scanCareLog(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, record)
	}
	return items, rows.Err()
}

func careExistingRanges(ctx context.Context, q careQuerier, user, exclude string) ([]careCycleRecord, error) {
	rows, err := q.Query(ctx, `SELECT start_date::text,COALESCE(end_date::text,'') FROM period_records WHERE user_id=$1 AND id::text<>$2 ORDER BY start_date`, user, exclude)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []careCycleRecord{}
	for rows.Next() {
		var record careCycleRecord
		if err = rows.Scan(&record.StartDate, &record.EndDate); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func careGrantedScopes(ctx context.Context, q careQuerier, owner, partner, coupleID string) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT scope FROM care_share_permissions
  WHERE owner_user_id=$1 AND partner_user_id=$2 AND couple_id=$3 AND revoked_at IS NULL ORDER BY granted_at,scope`, owner, partner, coupleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scopes := []string{}
	for rows.Next() {
		var scope string
		if err = rows.Scan(&scope); err != nil {
			return nil, err
		}
		scopes = append(scopes, scope)
	}
	return scopes, rows.Err()
}

func carePartnerPeriods(ctx context.Context, q careQuerier, user string) ([]carePartnerPeriod, error) {
	rows, err := q.Query(ctx, `SELECT id::text,start_date::text,end_date::text FROM period_records WHERE user_id=$1 ORDER BY start_date DESC,id LIMIT `+strconv.Itoa(careMaxPeriods), user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []carePartnerPeriod{}
	for rows.Next() {
		var record carePartnerPeriod
		if err = rows.Scan(&record.ID, &record.StartDate, &record.EndDate); err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	return items, rows.Err()
}

func careNotBound(err error) bool {
	var p *problem
	return errors.As(err, &p) && p.status == 403 && p.code == "not_bound"
}

// activeCouple resolves the caller's current relationship. Membership is checked
// again by chatTransaction where the operation also needs the couple lock.
func (s *Server) activeCouple(ctx context.Context, user string) (string, string, error) {
	var coupleID, peer string
	err := s.db.QueryRow(ctx, `SELECT c.id::text,
  CASE WHEN c.user_a=$1 THEN c.user_b::text ELSE c.user_a::text END
  FROM couples c JOIN couple_members m ON m.couple_id=c.id
  WHERE m.user_id=$1 AND c.status='active' LIMIT 1`, user).Scan(&coupleID, &peer)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", fail(403, "not_bound")
	}
	if err != nil {
		return "", "", err
	}
	return coupleID, peer, nil
}

func (s *Server) careGetProfile(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	profile, err := scanCareProfile(r.Context(), s.db, id.User.ID)
	if err != nil {
		return err
	}
	write(w, 200, map[string]any{"profile": profile})
	return nil
}

func (s *Server) carePutProfile(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	var input struct {
		Timezone          *string `json:"timezone"`
		AverageCycleDays  *int    `json:"average_cycle_days"`
		AveragePeriodDays *int    `json:"average_period_days"`
		ReminderEnabled   *bool   `json:"reminder_enabled"`
	}
	if err = decodeLimit(w, r, &input, 4<<10); err != nil {
		return err
	}
	profile, err := scanCareProfile(r.Context(), s.db, id.User.ID)
	if err != nil {
		return err
	}
	if input.Timezone != nil {
		if *input.Timezone == "" {
			return fail(400, "invalid_input")
		}
		if _, err = time.LoadLocation(*input.Timezone); err != nil {
			return fail(400, "invalid_input")
		}
		profile.Timezone = *input.Timezone
	}
	if input.AverageCycleDays != nil {
		profile.AverageCycleDays = *input.AverageCycleDays
	}
	if input.AveragePeriodDays != nil {
		profile.AveragePeriodDays = *input.AveragePeriodDays
	}
	if input.ReminderEnabled != nil {
		profile.ReminderEnabled = *input.ReminderEnabled
	}
	if profile.AverageCycleDays < careMinCycleDays || profile.AverageCycleDays > careMaxCycleDays ||
		profile.AveragePeriodDays < careMinPeriodDays || profile.AveragePeriodDays > careMaxPeriodDays {
		return fail(400, "invalid_input")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `INSERT INTO cycle_profiles(user_id,timezone,average_cycle_days,average_period_days,reminder_enabled)
  VALUES($1,$2,$3,$4,$5)
  ON CONFLICT(user_id) DO UPDATE SET timezone=excluded.timezone,average_cycle_days=excluded.average_cycle_days,
  average_period_days=excluded.average_period_days,reminder_enabled=excluded.reminder_enabled,updated_at=now()`,
		id.User.ID, profile.Timezone, profile.AverageCycleDays, profile.AveragePeriodDays, profile.ReminderEnabled); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]any{"profile": profile})
	return nil
}

func (s *Server) careListPeriods(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if (from != "" && !validCareDate(from)) || (to != "" && !validCareDate(to)) {
		return fail(400, "invalid_input")
	}
	items, err := listCarePeriods(r.Context(), s.db, id.User.ID, from, to)
	if err != nil {
		return err
	}
	write(w, 200, map[string]any{"periods": items})
	return nil
}

func (s *Server) careSavePeriod(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	create := r.Method == http.MethodPost
	recordID := r.PathValue("id")
	var input struct {
		ID        string `json:"id"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
		FlowLevel string `json:"flow_level"`
		Note      string `json:"note"`
	}
	if err = decodeLimit(w, r, &input, 8<<10); err != nil {
		return err
	}
	if create && recordID == "" {
		recordID = input.ID
	}
	if recordID == "" {
		if recordID, err = identifier(); err != nil {
			return err
		}
	} else if !uuidPattern.MatchString(recordID) {
		return fail(400, "invalid_input")
	}
	if !validCareDate(input.StartDate) {
		return fail(400, "invalid_input")
	}
	end := ""
	if input.EndDate != "" {
		if !validCareDate(input.EndDate) || input.EndDate < input.StartDate {
			return fail(400, "invalid_input")
		}
		end = input.EndDate
	}
	flow := input.FlowLevel
	if flow == "" {
		flow = "medium"
	}
	if !careContains(careFlowLevels, flow) || !validCareText(input.Note, careMaxNoteRunes) {
		return fail(400, "invalid_input")
	}
	profile, err := scanCareProfile(r.Context(), s.db, id.User.ID)
	if err != nil {
		return err
	}
	zone, err := time.LoadLocation(profile.Timezone)
	if err != nil {
		return err
	}
	today := time.Now().In(zone).Format(time.DateOnly)
	if input.StartDate > today || (end != "" && end > today) {
		return fail(400, "invalid_input")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	// One user's writes are serialised so two overlapping periods cannot both
	// pass the in-transaction overlap check; the unique index still catches
	// duplicate starts created outside this lock.
	if err = lockUsers(r.Context(), tx, id.User.ID); err != nil {
		return err
	}
	existing, err := careExistingRanges(r.Context(), tx, id.User.ID, recordID)
	if err != nil {
		return err
	}
	overlap, err := careOverlapsExisting(existing, input.StartDate, end)
	if err != nil {
		return err
	}
	if overlap {
		return fail(409, "cycle_overlap")
	}
	var endValue *string
	if end != "" {
		endValue = &end
	}
	if create {
		var count int
		if err = tx.QueryRow(r.Context(), `SELECT count(*) FROM period_records WHERE user_id=$1`, id.User.ID).Scan(&count); err != nil {
			return err
		}
		if count >= careMaxPeriods {
			return fail(400, "care_limit")
		}
		if _, err = tx.Exec(r.Context(), `INSERT INTO period_records(id,user_id,start_date,end_date,flow_level,note)
  VALUES($1,$2,$3::date,$4::date,$5,$6)`, recordID, id.User.ID, input.StartDate, endValue, flow, input.Note); err != nil {
			return err
		}
	} else {
		tag, execErr := tx.Exec(r.Context(), `UPDATE period_records SET start_date=$3::date,end_date=$4::date,flow_level=$5,note=$6,updated_at=now()
  WHERE id=$1 AND user_id=$2`, recordID, id.User.ID, input.StartDate, endValue, flow, input.Note)
		if execErr != nil {
			return execErr
		}
		if tag.RowsAffected() != 1 {
			return fail(404, "period_not_found")
		}
	}
	record, err := scanCarePeriod(tx.QueryRow(r.Context(), `SELECT `+carePeriodColumns+` FROM period_records WHERE id=$1 AND user_id=$2`, recordID, id.User.ID))
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
	write(w, status, map[string]any{"period": record})
	return nil
}

func (s *Server) careDeletePeriod(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	recordID := r.PathValue("id")
	if !uuidPattern.MatchString(recordID) {
		return fail(400, "invalid_input")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	tag, err := tx.Exec(r.Context(), `DELETE FROM period_records WHERE id=$1 AND user_id=$2`, recordID, id.User.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fail(404, "period_not_found")
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]bool{"ok": true})
	return nil
}

func (s *Server) careListLogs(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if (from != "" && !validCareDate(from)) || (to != "" && !validCareDate(to)) {
		return fail(400, "invalid_input")
	}
	items, err := listCareLogs(r.Context(), s.db, id.User.ID, from, to, careMaxLogs)
	if err != nil {
		return err
	}
	write(w, 200, map[string]any{"logs": items})
	return nil
}

func (s *Server) careSaveLog(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	create := r.Method == http.MethodPost
	recordID := r.PathValue("id")
	var input struct {
		ID        string   `json:"id"`
		LogDate   string   `json:"log_date"`
		PainLevel int      `json:"pain_level"`
		Mood      string   `json:"mood"`
		Symptoms  []string `json:"symptoms"`
		Note      string   `json:"note"`
	}
	if err = decodeLimit(w, r, &input, 8<<10); err != nil {
		return err
	}
	if create && recordID == "" {
		recordID = input.ID
	}
	if recordID == "" {
		if recordID, err = identifier(); err != nil {
			return err
		}
	} else if !uuidPattern.MatchString(recordID) {
		return fail(400, "invalid_input")
	}
	if !validCareDate(input.LogDate) || input.PainLevel < 0 || input.PainLevel > 10 || !careContains(careMoodLevels, input.Mood) || !validCareText(input.Note, careMaxNoteRunes) {
		return fail(400, "invalid_input")
	}
	symptoms, ok := normalizeCareSymptoms(input.Symptoms)
	if !ok {
		return fail(400, "invalid_input")
	}
	profile, err := scanCareProfile(r.Context(), s.db, id.User.ID)
	if err != nil {
		return err
	}
	zone, err := time.LoadLocation(profile.Timezone)
	if err != nil {
		return err
	}
	if input.LogDate > time.Now().In(zone).Format(time.DateOnly) {
		return fail(400, "invalid_input")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if err = lockUsers(r.Context(), tx, id.User.ID); err != nil {
		return err
	}
	var duplicate bool
	if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM cycle_daily_logs WHERE user_id=$1 AND log_date=$2::date AND id::text<>$3)`, id.User.ID, input.LogDate, recordID).Scan(&duplicate); err != nil {
		return err
	}
	if duplicate {
		return fail(409, "conflict")
	}
	if create {
		var count int
		if err = tx.QueryRow(r.Context(), `SELECT count(*) FROM cycle_daily_logs WHERE user_id=$1`, id.User.ID).Scan(&count); err != nil {
			return err
		}
		if count >= careMaxLogs {
			return fail(400, "care_limit")
		}
		if _, err = tx.Exec(r.Context(), `INSERT INTO cycle_daily_logs(id,user_id,log_date,pain_level,mood,symptoms,note)
  VALUES($1,$2,$3::date,$4,$5,$6,$7)`, recordID, id.User.ID, input.LogDate, input.PainLevel, input.Mood, symptoms, input.Note); err != nil {
			return err
		}
	} else {
		tag, execErr := tx.Exec(r.Context(), `UPDATE cycle_daily_logs SET log_date=$3::date,pain_level=$4,mood=$5,symptoms=$6,note=$7,updated_at=now()
  WHERE id=$1 AND user_id=$2`, recordID, id.User.ID, input.LogDate, input.PainLevel, input.Mood, symptoms, input.Note)
		if execErr != nil {
			return execErr
		}
		if tag.RowsAffected() != 1 {
			return fail(404, "log_not_found")
		}
	}
	record, err := scanCareLog(tx.QueryRow(r.Context(), `SELECT `+careLogColumns+` FROM cycle_daily_logs WHERE id=$1 AND user_id=$2`, recordID, id.User.ID))
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
	write(w, status, map[string]any{"log": record})
	return nil
}

func (s *Server) careDeleteLog(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	recordID := r.PathValue("id")
	if !uuidPattern.MatchString(recordID) {
		return fail(400, "invalid_input")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	tag, err := tx.Exec(r.Context(), `DELETE FROM cycle_daily_logs WHERE id=$1 AND user_id=$2`, recordID, id.User.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fail(404, "log_not_found")
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]bool{"ok": true})
	return nil
}

func (s *Server) careStatistics(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	profile, err := scanCareProfile(r.Context(), s.db, id.User.ID)
	if err != nil {
		return err
	}
	zone, err := time.LoadLocation(profile.Timezone)
	if err != nil {
		return err
	}
	periods, err := listCarePeriods(r.Context(), s.db, id.User.ID, "", "")
	if err != nil {
		return err
	}
	statistics, err := computeCareStatistics(careCycleRecords(periods), profile.AverageCycleDays, profile.AveragePeriodDays, time.Now().In(zone))
	if err != nil {
		return err
	}
	write(w, 200, map[string]any{"statistics": statistics})
	return nil
}

func (s *Server) careGetPreferences(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	preferences, err := scanCarePreferences(r.Context(), s.db, id.User.ID)
	if err != nil {
		return err
	}
	write(w, 200, map[string]any{"preferences": preferences})
	return nil
}

func (s *Server) carePutPreferences(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	var input struct {
		PreferredSupport []string `json:"preferred_support"`
		AvoidSupport     []string `json:"avoid_support"`
		Note             string   `json:"note"`
	}
	if err = decodeLimit(w, r, &input, 8<<10); err != nil {
		return err
	}
	preferred, ok := normalizeCareTags(input.PreferredSupport)
	if !ok {
		return fail(400, "invalid_input")
	}
	avoid, ok := normalizeCareTags(input.AvoidSupport)
	if !ok {
		return fail(400, "invalid_input")
	}
	if !validCareText(input.Note, careMaxPreferenceNote) {
		return fail(400, "invalid_input")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `INSERT INTO care_preferences(user_id,preferred_support,avoid_support,note)
  VALUES($1,$2,$3,$4)
  ON CONFLICT(user_id) DO UPDATE SET preferred_support=excluded.preferred_support,avoid_support=excluded.avoid_support,note=excluded.note,updated_at=now()`,
		id.User.ID, preferred, avoid, input.Note); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	preferences := carePreferences{PreferredSupport: preferred, AvoidSupport: avoid, Note: input.Note}
	write(w, 200, map[string]any{"preferences": preferences})
	return nil
}

func (s *Server) careListShares(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	rows, err := s.db.Query(r.Context(), `SELECT p.id::text,p.scope,p.granted_at FROM care_share_permissions p
  JOIN couples c ON c.id=p.couple_id AND c.status='active'
  WHERE p.owner_user_id=$1 AND p.revoked_at IS NULL ORDER BY p.granted_at,p.scope`, id.User.ID)
	if err != nil {
		return err
	}
	shares := []careShare{}
	for rows.Next() {
		var share careShare
		if err = rows.Scan(&share.ID, &share.Scope, &share.GrantedAt); err != nil {
			rows.Close()
			return err
		}
		shares = append(shares, share)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	// capabilities reports what the partner already shares with the caller, so
	// the settings page can show both directions without extra calls.
	capabilities := []string{}
	coupleID, peer, coupleErr := s.activeCouple(r.Context(), id.User.ID)
	if coupleErr == nil {
		scopes, scopeErr := careGrantedScopes(r.Context(), s.db, peer, id.User.ID, coupleID)
		if scopeErr != nil {
			return scopeErr
		}
		capabilities = scopes
	} else if !careNotBound(coupleErr) {
		return coupleErr
	}
	write(w, 200, map[string]any{"shares": shares, "scopes": careScopeList, "capabilities": capabilities})
	return nil
}

func (s *Server) careGrantShare(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	var input struct {
		Scope string `json:"scope"`
	}
	if err = decodeLimit(w, r, &input, 1<<10); err != nil {
		return err
	}
	if !careScopeValid(input.Scope) {
		return fail(400, "invalid_input")
	}
	coupleID, peer, err := s.activeCouple(r.Context(), id.User.ID)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	share, existing, err := scanCareShare(r.Context(), tx, id.User.ID, peer, coupleID, input.Scope)
	if err != nil {
		return err
	}
	if existing {
		write(w, 200, map[string]any{"share": share})
		return nil
	}
	shareID, err := identifier()
	if err != nil {
		return err
	}
	tag, err := tx.Exec(r.Context(), `INSERT INTO care_share_permissions(id,owner_user_id,partner_user_id,couple_id,scope)
  VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, shareID, id.User.ID, peer, coupleID, input.Scope)
	if err != nil {
		return err
	}
	created := tag.RowsAffected() == 1
	if !created {
		// A concurrent request granted the same scope; return that row instead.
		if share, _, err = scanCareShare(r.Context(), tx, id.User.ID, peer, coupleID, input.Scope); err != nil {
			return err
		}
	} else {
		share = careShare{ID: shareID, Scope: input.Scope}
		if err = tx.QueryRow(r.Context(), `SELECT granted_at FROM care_share_permissions WHERE id=$1`, shareID).Scan(&share.GrantedAt); err != nil {
			return err
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	status := 200
	if created {
		status = 201
	}
	write(w, status, map[string]any{"share": share})
	return nil
}

// scanCareShare returns the active grant for one owner/partner/scope triple and
// whether it already existed.
func scanCareShare(ctx context.Context, tx pgx.Tx, owner, partner, coupleID, scope string) (careShare, bool, error) {
	var share careShare
	err := tx.QueryRow(ctx, `SELECT id::text,scope,granted_at FROM care_share_permissions
  WHERE owner_user_id=$1 AND partner_user_id=$2 AND couple_id=$3 AND scope=$4 AND revoked_at IS NULL`, owner, partner, coupleID, scope).
		Scan(&share.ID, &share.Scope, &share.GrantedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return careShare{}, false, nil
	}
	if err != nil {
		return careShare{}, false, err
	}
	return share, true, nil
}

func (s *Server) careRevokeShare(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	shareID := r.PathValue("id")
	if !uuidPattern.MatchString(shareID) {
		return fail(400, "invalid_input")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var owner string
	err = tx.QueryRow(r.Context(), `SELECT owner_user_id::text FROM care_share_permissions WHERE id=$1`, shareID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(404, "not_found")
	}
	if err != nil {
		return err
	}
	if owner != id.User.ID {
		return fail(403, "forbidden")
	}
	// Revocation is a soft delete so the audit trail survives; repeating it is
	// harmless.
	if _, err = tx.Exec(r.Context(), `UPDATE care_share_permissions SET revoked_at=now() WHERE id=$1 AND revoked_at IS NULL`, shareID); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]bool{"ok": true})
	return nil
}

func (s *Server) carePartner(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	coupleID, _, err := s.activeCouple(r.Context(), id.User.ID)
	if err != nil {
		return err
	}
	tx, _, peer, err := s.chatTransaction(r, coupleID)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	scopes, err := careGrantedScopes(r.Context(), tx, peer, id.User.ID, coupleID)
	if err != nil {
		return err
	}
	grants := activeCareScopes(scopes)
	// Without any grant the response must not even reveal that records exist.
	if len(scopes) == 0 {
		return fail(403, "partner_no_share")
	}
	var displayName string
	if err = tx.QueryRow(r.Context(), `SELECT display_name FROM users WHERE id=$1`, peer).Scan(&displayName); err != nil {
		return err
	}
	view := carePartnerView{DisplayName: displayName, Grants: grants}
	if careGranted(grants, "cycle_summary") {
		profile, profileErr := scanCareProfile(r.Context(), tx, peer)
		if profileErr != nil {
			return profileErr
		}
		zone, zoneErr := time.LoadLocation(profile.Timezone)
		if zoneErr != nil {
			return zoneErr
		}
		periods, listErr := listCarePeriods(r.Context(), tx, peer, "", "")
		if listErr != nil {
			return listErr
		}
		statistics, statsErr := computeCareStatistics(careCycleRecords(periods), profile.AverageCycleDays, profile.AveragePeriodDays, time.Now().In(zone))
		if statsErr != nil {
			return statsErr
		}
		view.Summary = &carePartnerSummary{PredictedStartDate: statistics.PredictedStartDate, DaysUntilNext: statistics.DaysUntilNext, Estimate: true}
	}
	if careGranted(grants, "period_dates") {
		periods, listErr := carePartnerPeriods(r.Context(), tx, peer)
		if listErr != nil {
			return listErr
		}
		view.Periods = periods
	}
	if careGranted(grants, "daily_logs") {
		logs, listErr := listCareLogs(r.Context(), tx, peer, "", "", careMaxLogs)
		if listErr != nil {
			return listErr
		}
		view.Logs = logs
	}
	if careGranted(grants, "care_preferences") {
		preferences, readErr := scanCarePreferences(r.Context(), tx, peer)
		if readErr != nil {
			return readErr
		}
		view.Preferences = &preferences
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]any{"partner": view})
	return nil
}

func (s *Server) careListTasks(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	coupleID, _, err := s.activeCouple(r.Context(), id.User.ID)
	if err != nil {
		return err
	}
	status := r.URL.Query().Get("status")
	if status != "" && status != "open" && status != "done" {
		return fail(400, "invalid_input")
	}
	tx, _, _, err := s.chatTransaction(r, coupleID)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	rows, err := tx.Query(r.Context(), `SELECT `+careTaskColumns+` FROM care_tasks
  WHERE couple_id=$1 AND ($2='' OR status=$2) ORDER BY (status='done'),due_at,created_at LIMIT `+strconv.Itoa(careMaxTasks), coupleID, status)
	if err != nil {
		return err
	}
	tasks := []careTask{}
	for rows.Next() {
		task, scanErr := scanCareTask(rows)
		if scanErr != nil {
			rows.Close()
			return scanErr
		}
		tasks = append(tasks, careTaskFor(task, id.User.ID))
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]any{"tasks": tasks})
	return nil
}

func (s *Server) careSaveTask(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	create := r.Method == http.MethodPost
	taskID := r.PathValue("id")
	var input struct {
		ID     string  `json:"id"`
		Title  *string `json:"title"`
		Note   *string `json:"note"`
		DueAt  *string `json:"due_at"`
		Status *string `json:"status"`
	}
	if err = decodeLimit(w, r, &input, 8<<10); err != nil {
		return err
	}
	if create && taskID == "" {
		taskID = input.ID
	}
	if taskID == "" {
		if taskID, err = identifier(); err != nil {
			return err
		}
	} else if !uuidPattern.MatchString(taskID) {
		return fail(400, "invalid_input")
	}
	coupleID, peer, err := s.activeCouple(r.Context(), id.User.ID)
	if err != nil {
		return err
	}
	tx, _, _, err := s.chatTransaction(r, coupleID)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if create {
		if input.Title == nil {
			return fail(400, "invalid_input")
		}
		title := strings.TrimSpace(*input.Title)
		if title == "" || !validCareText(title, careMaxTitleRunes) {
			return fail(400, "invalid_input")
		}
		note := ""
		if input.Note != nil {
			note = *input.Note
		}
		if !validCareText(note, careMaxNoteRunes) {
			return fail(400, "invalid_input")
		}
		due, parseErr := careDueTime(input.DueAt)
		if parseErr != nil {
			return parseErr
		}
		task, insertErr := scanCareTask(tx.QueryRow(r.Context(), `INSERT INTO care_tasks(id,couple_id,subject_user_id,created_by_user_id,title,note,due_at)
  VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING `+careTaskColumns, taskID, coupleID, peer, id.User.ID, title, note, due))
		if insertErr != nil {
			return insertErr
		}
		if err = tx.Commit(r.Context()); err != nil {
			return err
		}
		write(w, 201, map[string]any{"task": careTaskFor(task, id.User.ID)})
		return nil
	}
	existing, err := scanCareTask(tx.QueryRow(r.Context(), `SELECT `+careTaskColumns+` FROM care_tasks WHERE id=$1 AND couple_id=$2 FOR UPDATE`, taskID, coupleID))
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(404, "task_not_found")
	}
	if err != nil {
		return err
	}
	title, note, due, status := existing.Title, existing.Note, existing.DueAt, existing.Status
	if input.Title != nil {
		title = strings.TrimSpace(*input.Title)
		if title == "" || !validCareText(title, careMaxTitleRunes) {
			return fail(400, "invalid_input")
		}
	}
	if input.Note != nil {
		note = *input.Note
		if !validCareText(note, careMaxNoteRunes) {
			return fail(400, "invalid_input")
		}
	}
	if input.DueAt != nil {
		if due, err = careDueTime(input.DueAt); err != nil {
			return err
		}
	}
	if input.Status != nil {
		if *input.Status != "open" && *input.Status != "done" {
			return fail(400, "invalid_input")
		}
		status = *input.Status
	}
	completed := existing.CompletedAt
	if status == "done" {
		if completed == nil {
			value := time.Now().UTC()
			completed = &value
		}
	} else {
		completed = nil
	}
	task, err := scanCareTask(tx.QueryRow(r.Context(), `UPDATE care_tasks SET title=$3,note=$4,due_at=$5,status=$6,completed_at=$7,updated_at=now()
  WHERE id=$1 AND couple_id=$2 RETURNING `+careTaskColumns, taskID, coupleID, title, note, due, status, completed))
	if err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]any{"task": careTaskFor(task, id.User.ID)})
	return nil
}

// careDueTime parses an optional RFC3339 due timestamp; an empty value clears it.
func careDueTime(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*value))
	if err != nil {
		return nil, fail(400, "invalid_input")
	}
	return &parsed, nil
}

func (s *Server) careDeleteTask(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	taskID := r.PathValue("id")
	if !uuidPattern.MatchString(taskID) {
		return fail(400, "invalid_input")
	}
	coupleID, _, err := s.activeCouple(r.Context(), id.User.ID)
	if err != nil {
		return err
	}
	tx, _, _, err := s.chatTransaction(r, coupleID)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	tag, err := tx.Exec(r.Context(), `DELETE FROM care_tasks WHERE id=$1 AND couple_id=$2`, taskID, coupleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fail(404, "task_not_found")
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]bool{"ok": true})
	return nil
}

func (s *Server) careListReminders(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	// Only the reminder id, kind and time leave the server; the client renders
	// fixed wording so no health content is ever pushed.
	rows, err := s.db.Query(r.Context(), `SELECT r.id::text,r.kind,r.scheduled_at FROM cycle_reminders r
  WHERE r.user_id=$1 AND r.scheduled_at<=now() AND r.scheduled_at>=now()-interval '24 hours' AND r.expires_at>now()
  AND NOT EXISTS(SELECT 1 FROM cycle_reminder_receipts p WHERE p.reminder_id=r.id AND p.device_id=$2)
  ORDER BY r.scheduled_at LIMIT 20`, id.User.ID, id.DeviceID)
	if err != nil {
		return err
	}
	defer rows.Close()
	reminders := []map[string]any{}
	for rows.Next() {
		var reminderID, kind string
		var scheduled time.Time
		if err = rows.Scan(&reminderID, &kind, &scheduled); err != nil {
			return err
		}
		reminders = append(reminders, map[string]any{"id": reminderID, "kind": kind, "scheduled_at": scheduled})
	}
	if err = rows.Err(); err != nil {
		return err
	}
	write(w, 200, map[string]any{"reminders": reminders})
	return nil
}

func (s *Server) careAckReminder(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	var input struct {
		ID string `json:"id"`
	}
	if err = decodeLimit(w, r, &input, 1<<10); err != nil {
		return err
	}
	if !uuidPattern.MatchString(input.ID) {
		return fail(400, "invalid_input")
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	// Receipts are per device, so a second phone still sees the reminder.
	if _, err = tx.Exec(r.Context(), `INSERT INTO cycle_reminder_receipts(reminder_id,device_id)
  SELECT r.id,$2 FROM cycle_reminders r WHERE r.id=$1 AND r.user_id=$3 ON CONFLICT DO NOTHING`, input.ID, id.DeviceID, id.User.ID); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]bool{"ok": true})
	return nil
}

func (s *Server) careDashboard(w http.ResponseWriter, r *http.Request) error {
	id, err := s.auth(r)
	if err != nil {
		return err
	}
	ctx := r.Context()
	profile, err := scanCareProfile(ctx, s.db, id.User.ID)
	if err != nil {
		return err
	}
	zone, err := time.LoadLocation(profile.Timezone)
	if err != nil {
		return err
	}
	now := time.Now().In(zone)
	today := now.Format(time.DateOnly)
	periods, err := listCarePeriods(ctx, s.db, id.User.ID, "", "")
	if err != nil {
		return err
	}
	statistics, err := computeCareStatistics(careCycleRecords(periods), profile.AverageCycleDays, profile.AveragePeriodDays, now)
	if err != nil {
		return err
	}
	dashboard := careDashboard{
		Profile: profile,
		Prediction: careDashboardPrediction{
			StartDate: statistics.PredictedStartDate,
			DaysUntil: statistics.DaysUntilNext,
			Estimate:  true,
		},
		Shares: careDashboardShares{Active: []string{}, Available: []string{}},
	}
	todayValue, parseErr := time.Parse(time.DateOnly, today)
	if parseErr != nil {
		return parseErr
	}
	for index := range periods {
		start := periods[index].StartDate
		end := start
		if periods[index].EndDate != nil {
			end = *periods[index].EndDate
		}
		if start <= today && today <= end {
			startValue, startErr := time.Parse(time.DateOnly, start)
			if startErr != nil {
				return startErr
			}
			dashboard.Today.InPeriod = true
			dashboard.Today.PeriodDay = int(todayValue.Sub(startValue).Hours()/24) + 1
			break
		}
	}
	if err = s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cycle_daily_logs WHERE user_id=$1 AND log_date=$2::date)`, id.User.ID, today).Scan(&dashboard.Today.HasLog); err != nil {
		return err
	}
	coupleID, peer, coupleErr := s.activeCouple(ctx, id.User.ID)
	switch {
	case coupleErr == nil:
		partner, partnerErr := s.readCareDashboardPartner(ctx, id.User.ID, peer, coupleID)
		if partnerErr != nil {
			return partnerErr
		}
		dashboard.Partner = partner
		outbound, scopeErr := careGrantedScopes(ctx, s.db, id.User.ID, peer, coupleID)
		if scopeErr != nil {
			return scopeErr
		}
		active := activeCareScopes(outbound)
		for _, scope := range careScopeList {
			if active[scope] {
				dashboard.Shares.Active = append(dashboard.Shares.Active, scope)
			} else {
				dashboard.Shares.Available = append(dashboard.Shares.Available, scope)
			}
		}
		if err = s.db.QueryRow(ctx, `SELECT count(*) FROM care_tasks WHERE couple_id=$1 AND status='open'`, coupleID).Scan(&dashboard.Today.OpenTasks); err != nil {
			return err
		}
	case careNotBound(coupleErr):
		// The home screen must still render for an unbound account.
	default:
		return coupleErr
	}
	write(w, 200, map[string]any{"dashboard": dashboard})
	return nil
}

func (s *Server) readCareDashboardPartner(ctx context.Context, user, peer, coupleID string) (*careDashboardPartner, error) {
	scopes, err := careGrantedScopes(ctx, s.db, peer, user, coupleID)
	if err != nil {
		return nil, err
	}
	grants := activeCareScopes(scopes)
	var displayName string
	if err = s.db.QueryRow(ctx, `SELECT display_name FROM users WHERE id=$1`, peer).Scan(&displayName); err != nil {
		return nil, err
	}
	partner := &careDashboardPartner{DisplayName: displayName, Grants: grants}
	if careGranted(grants, "cycle_summary") {
		profile, profileErr := scanCareProfile(ctx, s.db, peer)
		if profileErr != nil {
			return nil, profileErr
		}
		zone, zoneErr := time.LoadLocation(profile.Timezone)
		if zoneErr != nil {
			return nil, zoneErr
		}
		periods, listErr := listCarePeriods(ctx, s.db, peer, "", "")
		if listErr != nil {
			return nil, listErr
		}
		statistics, statsErr := computeCareStatistics(careCycleRecords(periods), profile.AverageCycleDays, profile.AveragePeriodDays, time.Now().In(zone))
		if statsErr != nil {
			return nil, statsErr
		}
		partner.Summary = &carePartnerSummary{PredictedStartDate: statistics.PredictedStartDate, DaysUntilNext: statistics.DaysUntilNext, Estimate: true}
	}
	return partner, nil
}

func insertCareReminder(ctx context.Context, tx pgx.Tx, user, kind string, scheduled time.Time) error {
	id, err := identifier()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO cycle_reminders(id,user_id,kind,scheduled_at,expires_at) VALUES($1,$2,$3,$4,$5)
  ON CONFLICT DO NOTHING`, id, user, kind, scheduled, scheduled.Add(careReminderWindow)); err != nil {
		return err
	}
	return nil
}

// generateCareReminders is idempotent: the unique key on (user, kind,
// scheduled_at) makes repeated runs and concurrent workers safe.
func (s *Server) generateCareReminders(ctx context.Context, now time.Time) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	type candidate struct {
		user, timezone string
		cycle, period  int
	}
	rows, err := tx.Query(ctx, `SELECT p.user_id::text,p.timezone,p.average_cycle_days,p.average_period_days
  FROM cycle_profiles p WHERE p.reminder_enabled
  AND EXISTS(SELECT 1 FROM period_records r WHERE r.user_id=p.user_id)
  ORDER BY p.user_id LIMIT 500`)
	if err != nil {
		return err
	}
	candidates := []candidate{}
	for rows.Next() {
		var value candidate
		if err = rows.Scan(&value.user, &value.timezone, &value.cycle, &value.period); err != nil {
			rows.Close()
			return err
		}
		candidates = append(candidates, value)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, value := range candidates {
		zone, zoneErr := time.LoadLocation(value.timezone)
		if zoneErr != nil {
			return zoneErr
		}
		periods, listErr := listCarePeriods(ctx, tx, value.user, "", "")
		if listErr != nil {
			return listErr
		}
		statistics, statsErr := computeCareStatistics(careCycleRecords(periods), value.cycle, value.period, now.In(zone))
		if statsErr != nil {
			return statsErr
		}
		if statistics.PredictedStartDate != nil {
			predicted, parseErr := time.Parse(time.DateOnly, *statistics.PredictedStartDate)
			if parseErr != nil {
				return parseErr
			}
			lead := predicted.AddDate(0, 0, -careReminderLeadDays)
			scheduled := time.Date(lead.Year(), lead.Month(), lead.Day(), careReminderHour, 0, 0, 0, zone)
			if !scheduled.Before(now.Add(-careReminderWindow)) && !scheduled.After(now.Add(careReminderWindow)) {
				if err = insertCareReminder(ctx, tx, value.user, "period_upcoming", scheduled); err != nil {
					return err
				}
			}
		}
		local := now.In(zone)
		if local.Hour() < careLogNudgeHour {
			continue
		}
		var hasLog bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cycle_daily_logs WHERE user_id=$1 AND log_date=$2::date)`, value.user, local.Format(time.DateOnly)).Scan(&hasLog); err != nil {
			return err
		}
		if hasLog {
			continue
		}
		nudge := time.Date(local.Year(), local.Month(), local.Day(), careLogNudgeHour, 0, 0, 0, zone)
		if err = insertCareReminder(ctx, tx, value.user, "log_nudge", nudge); err != nil {
			return err
		}
	}
	// Bound delivery metadata: reminders older than the window are never shown,
	// and receipts must go first because of the foreign key.
	if _, err = tx.Exec(ctx, `DELETE FROM cycle_reminder_receipts WHERE reminder_id IN (SELECT id FROM cycle_reminders WHERE scheduled_at<$1)`, now.Add(-careReminderWindow)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM cycle_reminders WHERE scheduled_at<$1`, now.Add(-careReminderWindow)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Server) RunCareReminders(ctx context.Context) {
	timer := time.NewTicker(30 * time.Second)
	defer timer.Stop()
	for {
		run, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := s.generateCareReminders(run, time.Now())
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Print("care reminder generation failed; durable rows retained for retry")
		}
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}
