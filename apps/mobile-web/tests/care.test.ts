import test from 'node:test';
import assert from 'node:assert/strict';
import {
  FLOWS, MOODS, SYMPTOMS, addDays, average, cycleLengths, inRange, isPeriodStart, monthGrid, monthLabel,
  parseYmd, periodDays, periodLengths, statistics, validateLog, validatePeriod, ymd,
} from '../src/care/cycle.ts';
import type { CareProfile, PeriodRecord } from '../src/core.ts';

const profile: CareProfile = { timezone: 'Asia/Shanghai', average_cycle_days: 28, average_period_days: 5, reminder_enabled: true };

function period(start_date: string, end_date: string | null = null, flow_level: 'light' | 'medium' | 'heavy' = 'medium'): PeriodRecord {
  return { id: start_date, start_date, end_date, flow_level, note: '', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' };
}

test('parseYmd accepts only real YYYY-MM-DD calendar days', () => {
  const leap = parseYmd('2024-02-29');
  assert.ok(leap);
  assert.deepEqual([leap.getFullYear(), leap.getMonth() + 1, leap.getDate()], [2024, 2, 29]);
  for (const value of ['', '2026-2-8', '2026-02-30', '2026-13-01', '2026-00-10', '2025-02-29', '2026-02-28x', '2026/02/28', 'abc', '0000-01-01']) {
    assert.equal(parseYmd(value), null, value);
  }
});

test('ymd formats local calendar fields with zero padding', () => {
  assert.equal(ymd(new Date(2026, 0, 5)), '2026-01-05');
  assert.equal(ymd(new Date(2026, 10, 20)), '2026-11-20');
  assert.equal(ymd(new Date(Number.NaN)), '');
});

test('addDays crosses months, years and February 29', () => {
  assert.equal(addDays('2026-01-31', 1), '2026-02-01');
  assert.equal(addDays('2026-12-31', 1), '2027-01-01');
  assert.equal(addDays('2024-02-28', 1), '2024-02-29');
  assert.equal(addDays('2024-02-29', 1), '2024-03-01');
  assert.equal(addDays('2023-02-28', 1), '2023-03-01');
  assert.equal(addDays('2026-03-01', -1), '2026-02-28');
  assert.equal(addDays('2026-01-01', -1), '2025-12-31');
  assert.equal(addDays('2026-01-01', 365), '2027-01-01');
  assert.equal(addDays('not-a-date', 1), '');
});

test('monthGrid is six Monday-first weeks with neighbour padding', () => {
  const grid = monthGrid(2026, 11);
  assert.equal(grid.length, 42);
  assert.equal(grid[0], '2026-10-26');
  assert.equal(grid[41], '2026-12-06');
  for (let index = 0; index < grid.length; index += 7) assert.equal(parseYmd(grid[index])?.getDay(), 1);
  assert.equal(parseYmd(grid[6])?.getDay(), 0);
  const leap = monthGrid(2024, 2);
  assert.equal(leap.length, 42);
  assert.ok(leap.includes('2024-02-29'));
  assert.equal(leap[0], '2024-01-29');
  assert.equal(leap[41], '2024-03-10');
});

test('monthLabel renders both locales', () => {
  assert.equal(monthLabel(2026, 11, 'zh'), '2026年11月');
  assert.equal(monthLabel(2026, 11, 'en'), 'November 2026');
  assert.equal(monthLabel(2024, 3, 'en'), 'March 2024');
  assert.equal(monthLabel(2026, 0, 'zh'), '');
});

test('inRange treats a missing end as that single day', () => {
  assert.equal(inRange('2026-01-02', '2026-01-01', '2026-01-05'), true);
  assert.equal(inRange('2026-01-01', '2026-01-01', '2026-01-05'), true);
  assert.equal(inRange('2026-01-05', '2026-01-01', '2026-01-05'), true);
  assert.equal(inRange('2026-01-06', '2026-01-01', '2026-01-05'), false);
  assert.equal(inRange('2026-01-01', '2026-01-01', null), true);
  assert.equal(inRange('2026-01-02', '2026-01-01', null), false);
  assert.equal(inRange('2026-02-30', '2026-01-01', null), false);
  assert.equal(inRange('2026-01-01', '2026-01-25', '2026-01-01'), false);
});

test('periodDays expands intervals and period starts are exact', () => {
  const periods = [period('2026-01-01', '2026-01-03'), period('2026-02-01')];
  const days = periodDays(periods);
  assert.equal(days.size, 4);
  for (const day of ['2026-01-01', '2026-01-02', '2026-01-03', '2026-02-01']) assert.ok(days.has(day));
  assert.ok(!days.has('2026-01-04'));
  assert.equal(isPeriodStart(periods, '2026-01-02'), false);
  assert.equal(isPeriodStart(periods, '2026-02-01'), true);
});

test('cycleLengths sorts starts, dedupes them and keeps only 15..60 day gaps', () => {
  const periods = [period('2026-01-29'), period('2026-01-01'), period('2026-02-10'), period('2026-03-20'), period('2026-01-01')];
  assert.deepEqual(cycleLengths(periods), [28, 38]);
  assert.deepEqual(cycleLengths([]), []);
  assert.deepEqual(cycleLengths([period('2026-01-01'), period('2026-01-11')]), []);
});

test('periodLengths counts inclusive days and ignores open records', () => {
  assert.deepEqual(periodLengths([period('2026-01-01', '2026-01-05'), period('2026-02-01'), period('2026-03-01', '2026-02-01')]), [5]);
});

test('average rounds and reports null for an empty sample', () => {
  assert.equal(average([]), null);
  assert.equal(average([28]), 28);
  assert.equal(average([27, 28]), 28);
  assert.equal(average([28, 29]), 29);
  assert.equal(average([28, 30]), 29);
});

test('statistics on empty history falls back to the profile defaults', () => {
  const result = statistics([], profile, '2026-11-20');
  assert.equal(result.average_cycle_days, 28);
  assert.equal(result.average_period_days, 5);
  assert.deepEqual(result.cycles, []);
  assert.equal(result.predicted_start_date, null);
  assert.equal(result.predicted_end_date, null);
  assert.equal(result.days_until_next, null);
  assert.equal(result.confidence, 'estimate');
});

test('statistics on a single record uses the profile cycle and its own period length', () => {
  const result = statistics([period('2026-01-01', '2026-01-05')], profile, '2026-01-10');
  assert.equal(result.average_cycle_days, 28);
  assert.equal(result.average_period_days, 5);
  assert.deepEqual(result.cycles, [{ start_date: '2026-01-01', end_date: '2026-01-05', length_days: null }]);
  assert.equal(result.predicted_start_date, '2026-01-29');
  assert.equal(result.predicted_end_date, '2026-02-02');
  assert.equal(result.days_until_next, 19);
});

test('statistics averages several cycles and predicts from the latest start', () => {
  const periods = [period('2026-01-26', '2026-01-30'), period('2025-12-01', '2025-12-05'), period('2025-12-29', '2026-01-02')];
  const result = statistics(periods, profile, '2026-02-01');
  assert.equal(result.average_cycle_days, 28);
  assert.equal(result.average_period_days, 5);
  assert.deepEqual(result.cycles.map(entry => entry.start_date), ['2025-12-01', '2025-12-29', '2026-01-26']);
  assert.deepEqual(result.cycles.map(entry => entry.length_days), [28, 28, null]);
  assert.equal(result.predicted_start_date, '2026-02-23');
  assert.equal(result.predicted_end_date, '2026-02-27');
  assert.equal(result.days_until_next, 22);
});

test('statistics handles cross-year records', () => {
  const result = statistics([period('2025-12-20', '2025-12-25'), period('2026-01-17', '2026-01-22')], profile, '2026-01-20');
  assert.equal(result.average_cycle_days, 28);
  assert.equal(result.average_period_days, 6);
  assert.equal(result.predicted_start_date, '2026-02-14');
  assert.equal(result.predicted_end_date, '2026-02-19');
  assert.equal(result.days_until_next, 25);
});

test('statistics without end dates falls back to the profile period length', () => {
  const result = statistics([period('2026-01-01')], { ...profile, average_period_days: 6 }, '2026-01-05');
  assert.equal(result.average_period_days, 6);
  assert.deepEqual(result.cycles, [{ start_date: '2026-01-01', end_date: null, length_days: null }]);
  assert.equal(result.predicted_start_date, '2026-01-29');
  assert.equal(result.predicted_end_date, '2026-02-03');
  assert.equal(result.days_until_next, 24);
});

test('statistics ignores implausible gaps in the average but still shows them in history', () => {
  const short = statistics([period('2026-01-01'), period('2026-01-11')], profile, '2026-01-15');
  assert.equal(short.average_cycle_days, 28);
  assert.equal(short.predicted_start_date, '2026-02-08');
  assert.deepEqual(short.cycles.map(entry => entry.length_days), [10, null]);
  const past = statistics([period('2026-01-01')], profile, '2026-02-15');
  assert.equal(past.predicted_start_date, '2026-01-29');
  assert.equal(past.days_until_next, -17);
});

test('validatePeriod rejects bad ranges, future dates and out-of-range fields', () => {
  const today = '2026-11-20';
  assert.equal(validatePeriod({ start_date: '2026-11-01', end_date: '2026-11-05', flow_level: 'light', note: '' }, today), null);
  assert.equal(validatePeriod({ start_date: '2026-11-20' }, today), null);
  assert.equal(validatePeriod({ start_date: '2026-11-01', end_date: null }, today), null);
  assert.equal(validatePeriod({ start_date: '2026-11-05', end_date: '2026-11-01' }, today), 'invalid_input');
  assert.equal(validatePeriod({ start_date: '2026-11-21' }, today), 'invalid_input');
  assert.equal(validatePeriod({ start_date: '2026-11-19', end_date: '2026-11-21' }, today), 'invalid_input');
  assert.equal(validatePeriod({ start_date: '2026-02-30' }, today), 'invalid_input');
  assert.equal(validatePeriod({ start_date: '2026-11-01', flow_level: 'flood' }, today), 'invalid_input');
  assert.equal(validatePeriod({ start_date: '2026-11-01', note: 'x'.repeat(501) }, today), 'invalid_input');
  assert.equal(validatePeriod({ start_date: '2026-11-01', note: 'x'.repeat(500) }, today), null);
});

test('validateLog rejects future days, out-of-range pain and unknown enums', () => {
  const today = '2026-11-20';
  assert.equal(validateLog({ log_date: '2026-11-20', pain_level: 0, mood: '', symptoms: [], note: '' }, today), null);
  assert.equal(validateLog({ log_date: '2026-11-01', pain_level: 10, mood: 'calm', symptoms: ['cramps'] }, today), null);
  assert.equal(validateLog({ log_date: '2026-11-21' }, today), 'invalid_input');
  assert.equal(validateLog({ log_date: '2026-02-30' }, today), 'invalid_input');
  assert.equal(validateLog({ log_date: '2026-11-01', pain_level: -1 }, today), 'invalid_input');
  assert.equal(validateLog({ log_date: '2026-11-01', pain_level: 11 }, today), 'invalid_input');
  assert.equal(validateLog({ log_date: '2026-11-01', pain_level: 3.5 }, today), 'invalid_input');
  assert.equal(validateLog({ log_date: '2026-11-01', mood: 'angry' }, today), 'invalid_input');
  assert.equal(validateLog({ log_date: '2026-11-01', symptoms: ['fever'] }, today), 'invalid_input');
  assert.equal(validateLog({ log_date: '2026-11-01', note: 'x'.repeat(501) }, today), 'invalid_input');
});

test('mood, symptom and flow whitelists match the server contract', () => {
  assert.deepEqual(MOODS, ['happy', 'calm', 'tired', 'sad', 'irritable', 'anxious']);
  assert.deepEqual(SYMPTOMS, ['cramps', 'headache', 'fatigue', 'bloating', 'backache', 'insomnia']);
  assert.deepEqual(FLOWS, ['light', 'medium', 'heavy']);
  assert.ok(MOODS.every(value => validateLog({ log_date: '2026-11-20', mood: value }, '2026-11-20') === null));
  assert.ok(SYMPTOMS.every(value => validateLog({ log_date: '2026-11-20', symptoms: [value] }, '2026-11-20') === null));
  assert.ok(FLOWS.every(value => validatePeriod({ start_date: '2026-11-20', flow_level: value }, '2026-11-20') === null));
});
