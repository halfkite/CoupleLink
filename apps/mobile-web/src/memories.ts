import type { Anniversary } from './core';
// One minimum touch target is the tolerance for considering the list at its end.
export function nearLatest(scrollTop: number, scrollHeight: number, clientHeight: number) {
  return scrollHeight - clientHeight - scrollTop <= 48;
}
export function anniversaryDays(a: Pick<Anniversary, 'date' | 'recurrence' | 'timezone'>, now = new Date()): number {
  const parts = new Intl.DateTimeFormat('en', { timeZone: a.timezone, year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(now);
  const n = (type: string) => Number(parts.find(p => p.type === type)?.value);
  const today = Date.UTC(n('year'), n('month') - 1, n('day'));
  const [year, month, day] = a.date.split('-').map(Number) as [number, number, number];
  const occurrence = (y: number) => Date.UTC(y, month - 1, Math.min(day, new Date(Date.UTC(y, month, 0)).getUTCDate()));
  let target = occurrence(a.recurrence === 'yearly' ? Math.max(year, n('year')) : year);
  if (a.recurrence === 'yearly' && target < today) target = occurrence(Math.max(year, n('year')) + 1);
  return Math.round((target - today) / 86400000);
}
