import type { CareCycleEntry, CareProfile, CareStatistics, PeriodRecord } from '../core';

// The server stores these exact values; the UI maps them through strings.ts.
export const MOODS: string[] = ['happy', 'calm', 'tired', 'sad', 'irritable', 'anxious'];

export const SYMPTOMS: string[] = ['cramps', 'headache', 'fatigue', 'bloating', 'backache', 'insomnia'];

export const FLOWS: string[] = ['light', 'medium', 'heavy'];

const DAY_MS = 86400000;

const MONTHS_EN = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];

/** Longest span a single period record may cover when expanding its days. */
const MAX_PERIOD_SPAN = 400;

function pad(value: number, width = 2): string { return String(value).padStart(width, '0'); }

/** Parses YYYY-MM-DD into a UTC midnight timestamp; null on any malformed or impossible date. */
function utcOf(value: string): number | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const year = Number(match[1]), month = Number(match[2]), day = Number(match[3]);
  if (month < 1 || month > 12 || day < 1 || day > 31) return null;
  const time = Date.UTC(year, month - 1, day), date = new Date(time);
  if (date.getUTCFullYear() !== year || date.getUTCMonth() !== month - 1 || date.getUTCDate() !== day) return null;
  return time;
}

function isoOf(time: number): string {
  const date = new Date(time);
  return `${pad(date.getUTCFullYear(), 4)}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}`;
}

/** Formats a Date from its local calendar fields, so the day never shifts across timezones. */
export function ymd(date: Date): string {
  if (!Number.isFinite(date.getTime())) return '';
  return `${pad(date.getFullYear(), 4)}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

/** Strict YYYY-MM-DD parse: rejects short forms, trailing text and rolled-over days like 2026-02-30. */
export function parseYmd(value: string): Date | null {
  if (utcOf(value) === null) return null;
  const [year, month, day] = value.split('-').map(Number) as [number, number, number];
  return new Date(year, month - 1, day);
}

export function addDays(date: string, days: number): string {
  const time = utcOf(date);
  if (time === null || !Number.isFinite(days)) return '';
  return isoOf(time + Math.trunc(days) * DAY_MS);
}

/** Six Monday-first weeks covering the month, with leading and trailing days from neighbours. */
export function monthGrid(year: number, month: number): string[] {
  if (!Number.isSafeInteger(year) || !Number.isSafeInteger(month) || month < 1 || month > 12) return [];
  const first = isoOf(Date.UTC(year, month - 1, 1));
  const weekday = new Date(Date.UTC(year, month - 1, 1)).getUTCDay();
  const start = addDays(first, -((weekday + 6) % 7));
  if (!start) return [];
  const grid: string[] = [];
  for (let index = 0; index < 42; index++) grid.push(addDays(start, index));
  return grid;
}

export function monthLabel(year: number, month: number, locale: 'zh' | 'en'): string {
  if (!Number.isSafeInteger(year) || !Number.isSafeInteger(month) || month < 1 || month > 12) return '';
  return locale === 'en' ? `${MONTHS_EN[month - 1]} ${year}` : `${year}年${month}月`;
}

/** True when the date falls inside [start, end]; a missing end means that single day only. */
export function inRange(date: string, start: string, end: string | null): boolean {
  if (utcOf(date) === null || utcOf(start) === null) return false;
  const stop = end || null;
  if (stop !== null && utcOf(stop) === null) return false;
  return stop === null ? date === start : date >= start && date <= stop;
}

function sortedStarts(periods: PeriodRecord[]): string[] {
  const starts = new Set<string>();
  for (const period of periods) if (utcOf(period.start_date) !== null) starts.add(period.start_date);
  return [...starts].sort();
}

/** Every calendar day covered by the given periods, both ends included. */
export function periodDays(periods: PeriodRecord[]): Set<string> {
  const days = new Set<string>();
  for (const period of periods) {
    if (utcOf(period.start_date) === null) continue;
    const end = period.end_date || null;
    if (end === null || utcOf(end) === null || end < period.start_date) { days.add(period.start_date); continue; }
    let cursor = period.start_date;
    for (let guard = 0; cursor && cursor <= end && guard < MAX_PERIOD_SPAN; guard++) { days.add(cursor); cursor = addDays(cursor, 1); }
  }
  return days;
}

export function isPeriodStart(periods: PeriodRecord[], date: string): boolean {
  return periods.some(period => period.start_date === date);
}

/** Gaps between consecutive starts, keeping only plausible cycle lengths (15..60 days). */
export function cycleLengths(periods: PeriodRecord[]): number[] {
  const starts = sortedStarts(periods), lengths: number[] = [];
  for (let index = 1; index < starts.length; index++) {
    const previous = utcOf(starts[index - 1]), current = utcOf(starts[index]);
    if (previous === null || current === null) continue;
    const days = Math.round((current - previous) / DAY_MS);
    if (days >= 15 && days <= 60) lengths.push(days);
  }
  return lengths;
}

/** Length in days of every record that has a valid end date. */
export function periodLengths(periods: PeriodRecord[]): number[] {
  const lengths: number[] = [];
  for (const period of periods) {
    if (!period.end_date) continue;
    const start = utcOf(period.start_date), end = utcOf(period.end_date);
    if (start === null || end === null || end < start) continue;
    lengths.push(Math.round((end - start) / DAY_MS) + 1);
  }
  return lengths;
}

/** Rounded mean; null for an empty sample so callers can fall back to the profile default. */
export function average(values: number[]): number | null {
  if (!values.length) return null;
  return Math.round(values.reduce((sum, value) => sum + value, 0) / values.length);
}

/**
 * Mirrors the Go statistics endpoint: cycle lengths come from consecutive starts 15..60 apart,
 * period lengths from records with an end date, both falling back to the saved profile average.
 */
export function statistics(periods: PeriodRecord[], profile: CareProfile, today: string): CareStatistics {
  const starts = sortedStarts(periods);
  const byStart = new Map<string, PeriodRecord>();
  for (const period of periods) if (utcOf(period.start_date) !== null && !byStart.has(period.start_date)) byStart.set(period.start_date, period);

  const cycles: CareCycleEntry[] = starts.map((start, index) => {
    const period = byStart.get(start) as PeriodRecord;
    const next = starts[index + 1];
    let length: number | null = null;
    if (next) {
      const previousTime = utcOf(start), nextTime = utcOf(next);
      // History shows the real gap; only the average below filters implausible lengths.
      if (previousTime !== null && nextTime !== null) length = Math.round((nextTime - previousTime) / DAY_MS);
    }
    return { start_date: start, end_date: period.end_date || null, length_days: length };
  });

  const measuredCycle = average(cycleLengths(periods));
  const measuredPeriod = average(periodLengths(periods));
  const averageCycleDays = measuredCycle === null ? profile.average_cycle_days : measuredCycle;
  const averagePeriodDays = measuredPeriod === null ? profile.average_period_days : measuredPeriod;

  const lastStart = starts.length ? starts[starts.length - 1] : null;
  const predictedStart = lastStart ? addDays(lastStart, averageCycleDays) : '';
  const predictedEnd = predictedStart ? addDays(predictedStart, Math.max(1, averagePeriodDays) - 1) : '';
  const todayTime = utcOf(today), predictedTime = predictedStart ? utcOf(predictedStart) : null;

  return {
    average_cycle_days: averageCycleDays,
    average_period_days: averagePeriodDays,
    cycles,
    predicted_start_date: predictedStart || null,
    predicted_end_date: predictedEnd || null,
    days_until_next: predictedTime !== null && todayTime !== null ? Math.round((predictedTime - todayTime) / DAY_MS) : null,
    confidence: 'estimate',
  };
}

/** Returns a strings.ts error key for an unusable period input, or null when it can be saved. */
export function validatePeriod(input: { start_date: string; end_date?: string | null; flow_level?: string; note?: string }, today: string): string | null {
  const todayTime = utcOf(today), start = utcOf(input.start_date);
  if (todayTime === null || start === null || start > todayTime) return 'invalid_input';
  const endValue = input.end_date || null;
  if (endValue !== null) {
    const end = utcOf(endValue);
    if (end === null || end < start || end > todayTime) return 'invalid_input';
  }
  if (input.flow_level !== undefined && !FLOWS.includes(input.flow_level)) return 'invalid_input';
  if (input.note !== undefined && [...input.note].length > 500) return 'invalid_input';
  return null;
}

/** Returns a strings.ts error key for an unusable daily log input, or null when it can be saved. */
export function validateLog(input: { log_date: string; pain_level?: number; mood?: string; symptoms?: string[] | null; note?: string }, today: string): string | null {
  const day = utcOf(input.log_date), todayTime = utcOf(today);
  if (day === null || todayTime === null || day > todayTime) return 'invalid_input';
  if (input.pain_level !== undefined && (!Number.isInteger(input.pain_level) || input.pain_level < 0 || input.pain_level > 10)) return 'invalid_input';
  if (input.mood !== undefined && input.mood !== '' && !MOODS.includes(input.mood)) return 'invalid_input';
  if (input.symptoms != null && (!Array.isArray(input.symptoms) || input.symptoms.some(item => !SYMPTOMS.includes(item)))) return 'invalid_input';
  if (input.note !== undefined && [...input.note].length > 500) return 'invalid_input';
  return null;
}
