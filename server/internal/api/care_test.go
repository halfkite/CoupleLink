package api

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func careTestDate(value string) time.Time {
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func careTestRecords(values ...[2]string) []careCycleRecord {
	records := make([]careCycleRecord, 0, len(values))
	for _, value := range values {
		records = append(records, careCycleRecord{StartDate: value[0], EndDate: value[1]})
	}
	return records
}

func careTestString(value string) *string { return &value }

func carePointer(value int) *int { return &value }

func TestCareStatisticsFromHistory(t *testing.T) {
	cases := []struct {
		name    string
		records []careCycleRecord
		today   string
		cycle   int
		period  int
		start   *string
		end     *string
		days    *int
	}{
		{
			name:    "two regular cycles",
			records: careTestRecords([2]string{"2026-10-04", "2026-10-08"}, [2]string{"2026-11-01", "2026-11-05"}),
			today:   "2026-11-20", cycle: 28, period: 5,
			start: careTestString("2026-11-29"), end: careTestString("2026-12-03"), days: carePointer(9),
		},
		{
			name: "average is rounded half up",
			records: careTestRecords(
				[2]string{"2026-08-01", ""}, [2]string{"2026-08-28", ""}, [2]string{"2026-09-25", ""},
			),
			today: "2026-09-26", cycle: 28, period: 5,
			start: careTestString("2026-10-23"), end: careTestString("2026-10-27"), days: carePointer(27),
		},
		{
			name: "gaps outside 15 to 60 are ignored",
			records: careTestRecords(
				[2]string{"2026-01-01", ""}, [2]string{"2026-01-11", ""}, [2]string{"2026-02-20", ""},
			),
			today: "2026-02-21", cycle: 40, period: 5,
			start: careTestString("2026-04-01"), end: careTestString("2026-04-05"), days: carePointer(39),
		},
		{
			name:    "only unusable gaps fall back to the profile average",
			records: careTestRecords([2]string{"2026-01-01", ""}, [2]string{"2026-01-11", ""}),
			today:   "2026-01-12", cycle: 28, period: 5,
			start: careTestString("2026-02-08"), end: careTestString("2026-02-12"), days: carePointer(27),
		},
		{
			name:    "crosses the new year",
			records: careTestRecords([2]string{"2025-12-20", "2025-12-24"}, [2]string{"2026-01-17", "2026-01-21"}),
			today:   "2026-01-18", cycle: 28, period: 5,
			start: careTestString("2026-02-14"), end: careTestString("2026-02-18"), days: carePointer(27),
		},
		{
			name:    "leap day start",
			records: careTestRecords([2]string{"2024-02-29", "2024-03-04"}, [2]string{"2024-03-28", "2024-04-01"}),
			today:   "2024-03-30", cycle: 28, period: 5,
			start: careTestString("2024-04-25"), end: careTestString("2024-04-29"), days: carePointer(26),
		},
		{
			name:    "prediction in the past stays negative",
			records: careTestRecords([2]string{"2026-10-01", "2026-10-04"}),
			today:   "2026-11-20", cycle: 28, period: 4,
			start: careTestString("2026-10-29"), end: careTestString("2026-11-01"), days: carePointer(-22),
		},
		{
			name:    "average period rounds independently of the cycle",
			records: careTestRecords([2]string{"2026-10-01", "2026-10-04"}, [2]string{"2026-11-01", "2026-11-05"}),
			today:   "2026-11-20", cycle: 31, period: 5,
			start: careTestString("2026-12-02"), end: careTestString("2026-12-06"), days: carePointer(12),
		},
	}
	for _, testCase := range cases {
		statistics, err := computeCareStatistics(testCase.records, 28, 5, careTestDate(testCase.today))
		if err != nil {
			t.Fatalf("%s: %v", testCase.name, err)
		}
		if statistics.AverageCycleDays != testCase.cycle || statistics.AveragePeriodDays != testCase.period {
			t.Fatalf("%s: averages = %d/%d, want %d/%d", testCase.name, statistics.AverageCycleDays, statistics.AveragePeriodDays, testCase.cycle, testCase.period)
		}
		if statistics.Confidence != "estimate" {
			t.Fatalf("%s: confidence = %q", testCase.name, statistics.Confidence)
		}
		if statistics.PredictedStartDate == nil || *statistics.PredictedStartDate != *testCase.start {
			t.Fatalf("%s: predicted start = %v, want %q", testCase.name, statistics.PredictedStartDate, *testCase.start)
		}
		if statistics.PredictedEndDate == nil || *statistics.PredictedEndDate != *testCase.end {
			t.Fatalf("%s: predicted end = %v, want %q", testCase.name, statistics.PredictedEndDate, *testCase.end)
		}
		if statistics.DaysUntilNext == nil || *statistics.DaysUntilNext != *testCase.days {
			t.Fatalf("%s: days until next = %v, want %d", testCase.name, statistics.DaysUntilNext, *testCase.days)
		}
		if len(statistics.Cycles) != len(testCase.records) {
			t.Fatalf("%s: cycles = %d, want %d", testCase.name, len(statistics.Cycles), len(testCase.records))
		}
	}
}

func TestCareStatisticsEmptyAndSingleHistory(t *testing.T) {
	empty, err := computeCareStatistics(nil, 30, 6, careTestDate("2026-11-20"))
	if err != nil {
		t.Fatal(err)
	}
	if empty.PredictedStartDate != nil || empty.PredictedEndDate != nil || empty.DaysUntilNext != nil {
		t.Fatal("an empty history must not predict a date")
	}
	if empty.AverageCycleDays != 30 || empty.AveragePeriodDays != 6 {
		t.Fatal("an empty history must fall back to the profile defaults")
	}
	if empty.Cycles == nil || len(empty.Cycles) != 0 {
		t.Fatal("cycles must marshal as an empty array")
	}
	// A single record still seeds a prediction from the profile average; the
	// client shows it as an estimate.
	single, err := computeCareStatistics(careTestRecords([2]string{"2026-11-01", "2026-11-04"}), 29, 4, careTestDate("2026-11-20"))
	if err != nil {
		t.Fatal(err)
	}
	if single.PredictedStartDate == nil || *single.PredictedStartDate != "2026-11-30" {
		t.Fatalf("single record prediction = %v", single.PredictedStartDate)
	}
	if single.PredictedEndDate == nil || *single.PredictedEndDate != "2026-12-03" {
		t.Fatalf("single record predicted end = %v", single.PredictedEndDate)
	}
	if single.DaysUntilNext == nil || *single.DaysUntilNext != 10 {
		t.Fatalf("single record days until next = %v", single.DaysUntilNext)
	}
	if len(single.Cycles) != 1 || single.Cycles[0].LengthDays != nil {
		t.Fatal("the newest cycle must report a null length")
	}
}

func TestCareStatisticsCycleLengths(t *testing.T) {
	statistics, err := computeCareStatistics(careTestRecords(
		[2]string{"2026-01-01", "2026-01-05"}, [2]string{"2026-01-29", ""}, [2]string{"2026-02-26", "2026-03-02"},
	), 28, 5, careTestDate("2026-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	lengths := []*int{statistics.Cycles[0].LengthDays, statistics.Cycles[1].LengthDays, statistics.Cycles[2].LengthDays}
	if lengths[0] == nil || *lengths[0] != 28 || lengths[1] == nil || *lengths[1] != 28 {
		t.Fatal("cycle lengths must be the distance to the next start")
	}
	if lengths[2] != nil {
		t.Fatal("the last cycle length must be null")
	}
	if statistics.Cycles[0].EndDate == nil || *statistics.Cycles[0].EndDate != "2026-01-05" {
		t.Fatal("cycle end date must come from the record")
	}
	if statistics.Cycles[1].EndDate != nil {
		t.Fatal("an unfinished period must report a null end date")
	}
}

func TestCareDateValidation(t *testing.T) {
	for _, value := range []string{"2026-11-20", "2024-02-29", "2026-01-01", "2026-12-31"} {
		if !validCareDate(value) {
			t.Fatalf("%q must be accepted", value)
		}
	}
	for _, value := range []string{"", "2026-1-1", "2026-13-01", "2026-02-30", "2025-02-29", "2026-11-20T00:00:00Z", "not a date", "0000-01-01"} {
		if validCareDate(value) {
			t.Fatalf("%q must be rejected", value)
		}
	}
	if "2026-11-21" <= "2026-11-20" {
		t.Fatal("date strings must compare chronologically")
	}
	if "2026-11-19" > "2026-11-20" {
		t.Fatal("date strings must compare chronologically")
	}
}

func TestCareEnumerationsAndTextBounds(t *testing.T) {
	for _, value := range []string{"", "happy", "calm", "tired", "sad", "irritable", "anxious"} {
		if !careContains(careMoodLevels, value) {
			t.Fatalf("mood %q must be accepted", value)
		}
	}
	for _, value := range []string{"angry", "HAPPY", "happy "} {
		if careContains(careMoodLevels, value) {
			t.Fatalf("mood %q must be rejected", value)
		}
	}
	for _, value := range []string{"light", "medium", "heavy"} {
		if !careContains(careFlowLevels, value) {
			t.Fatalf("flow %q must be accepted", value)
		}
	}
	if careContains(careFlowLevels, "extreme") {
		t.Fatal("unknown flow level accepted")
	}
	if careMinCycleDays != 21 || careMaxCycleDays != 45 || careMinPeriodDays != 1 || careMaxPeriodDays != 10 {
		t.Fatal("documented cycle bounds must stay 21..45 and 1..10")
	}
	if careMaxPeriods != 400 || careMaxLogs != 500 {
		t.Fatal("documented record limits must stay 400/500")
	}
	if !validCareText(strings.Repeat("a", careMaxNoteRunes), careMaxNoteRunes) {
		t.Fatal("a maximum length note must be accepted")
	}
	if validCareText(strings.Repeat("a", careMaxNoteRunes+1), careMaxNoteRunes) {
		t.Fatal("an oversized note must be rejected")
	}
	if validCareText("bad\x00text", careMaxNoteRunes) {
		t.Fatal("NUL bytes must be rejected")
	}
	if !validCareText("", careMaxNoteRunes) {
		t.Fatal("an empty note is the default and must be accepted")
	}
	if !validCareText(strings.Repeat("水", careMaxPreferenceNote), careMaxPreferenceNote) {
		t.Fatal("rune counting must not be byte counting")
	}
}

func TestCareSymptoms(t *testing.T) {
	symptoms, ok := normalizeCareSymptoms(nil)
	if !ok || symptoms == nil || len(symptoms) != 0 {
		t.Fatal("no symptoms is the default and must stay an empty array")
	}
	symptoms, ok = normalizeCareSymptoms([]string{"cramps", "headache", "cramps"})
	if !ok || len(symptoms) != 2 || symptoms[0] != "cramps" || symptoms[1] != "headache" {
		t.Fatalf("duplicates must collapse in order, got %v", symptoms)
	}
	// The whitelist itself bounds the count; the explicit limit is a guard in
	// case the whitelist grows, so a longer input must still collapse.
	repeated := make([]string, 0, len(careSymptomList)+1)
	for index := 0; index <= len(careSymptomList); index++ {
		repeated = append(repeated, careSymptomList[index%len(careSymptomList)])
	}
	symptoms, ok = normalizeCareSymptoms(repeated)
	if !ok || len(symptoms) != len(careSymptomList) {
		t.Fatalf("repeated symptoms must collapse to the whitelist, got %v ok=%v", symptoms, ok)
	}
	if _, ok = normalizeCareSymptoms([]string{"cramps", "fever"}); ok {
		t.Fatal("symptoms outside the whitelist must be rejected")
	}
}

func TestCareScopesAuthorizeOnlyTheWhitelist(t *testing.T) {
	for _, scope := range careScopeList {
		if !careScopeValid(scope) {
			t.Fatalf("scope %q must be accepted", scope)
		}
	}
	for _, scope := range []string{"", "everything", "Cycle_Summary", "daily-logs"} {
		if careScopeValid(scope) {
			t.Fatalf("scope %q must be rejected", scope)
		}
	}
	active := activeCareScopes([]string{"cycle_summary", "bogus"})
	if !careGranted(active, "cycle_summary") {
		t.Fatal("a granted scope must be active")
	}
	if careGranted(active, "bogus") || careGranted(active, "period_dates") {
		t.Fatal("unknown or ungranted scopes must not be active")
	}
	// A revoked scope disappears from the active list and therefore stops
	// authorising access immediately.
	revoked := activeCareScopes([]string{"cycle_summary"})
	delete(revoked, "cycle_summary")
	if careGranted(revoked, "cycle_summary") {
		t.Fatal("a revoked scope must stop authorising access")
	}
}

func TestCareRangesOverlap(t *testing.T) {
	cases := []struct {
		name         string
		aStart, aEnd string
		bStart, bEnd string
		want         bool
	}{
		{"same day", "2026-11-01", "2026-11-05", "2026-11-01", "2026-11-05", true},
		{"second range inside the first", "2026-11-01", "2026-11-05", "2026-11-02", "2026-11-03", true},
		{"open ended start day inside", "2026-11-01", "", "2026-11-01", "2026-11-03", true},
		{"adjacent days do not overlap", "2026-11-01", "2026-11-05", "2026-11-06", "2026-11-08", false},
		{"disjoint", "2026-11-01", "2026-11-05", "2026-12-01", "2026-12-03", false},
		{"single day equal", "2026-11-01", "", "2026-11-01", "", true},
		{"single day different", "2026-11-01", "", "2026-11-02", "", false},
		{"straddles across a month boundary", "2026-10-30", "2026-11-02", "2026-11-02", "2026-11-04", true},
	}
	for _, testCase := range cases {
		got, err := careRangesOverlap(testCase.aStart, testCase.aEnd, testCase.bStart, testCase.bEnd)
		if err != nil {
			t.Fatalf("%s: %v", testCase.name, err)
		}
		if got != testCase.want {
			t.Fatalf("%s: overlap = %v, want %v", testCase.name, got, testCase.want)
		}
	}
	if _, err := careRangesOverlap("2026-02-30", "", "2026-01-01", ""); err == nil {
		t.Fatal("invalid dates must not silently compare")
	}
	existing := careTestRecords([2]string{"2026-11-01", "2026-11-05"}, [2]string{"2026-12-01", ""})
	if overlap, err := careOverlapsExisting(existing, "2026-11-04", "2026-11-06"); err != nil || !overlap {
		t.Fatalf("existing range must be detected, overlap=%v err=%v", overlap, err)
	}
	if overlap, err := careOverlapsExisting(existing, "2026-11-06", "2026-11-08"); err != nil || overlap {
		t.Fatalf("a following range must be allowed, overlap=%v err=%v", overlap, err)
	}
	if overlap, err := careOverlapsExisting(existing, "2026-12-01", ""); err != nil || !overlap {
		t.Fatal("an open ended record must cover its start day")
	}
}

func TestCarePreferenceTags(t *testing.T) {
	tags, ok := normalizeCareTags(nil)
	if !ok || tags == nil || len(tags) != 0 {
		t.Fatal("no tags must stay an empty array")
	}
	tags, ok = normalizeCareTags([]string{" 抱抱 ", "抱抱", "说教"})
	if !ok || len(tags) != 2 || tags[0] != "抱抱" || tags[1] != "说教" {
		t.Fatalf("tags must be trimmed and deduplicated, got %v", tags)
	}
	if _, ok = normalizeCareTags([]string{""}); ok {
		t.Fatal("empty tags must be rejected")
	}
	over := make([]string, 0, careMaxTags+1)
	for index := 0; index <= careMaxTags; index++ {
		over = append(over, "tag-"+strconv.Itoa(index))
	}
	if _, ok = normalizeCareTags(over); ok {
		t.Fatal("too many tags must be rejected")
	}
}
