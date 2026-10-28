import test from 'node:test';
import assert from 'node:assert/strict';
import { anniversaryDays, nearLatest } from '../src/memories.ts';
test('anniversary countdown uses relationship timezone and preserves February 29', () => {
 const a = { date: '2024-02-29', recurrence: 'yearly' as const, timezone: 'Asia/Shanghai' };
 assert.equal(anniversaryDays(a, new Date('2025-02-27T18:00:00Z')), 0);
 assert.equal(anniversaryDays(a, new Date('2027-03-01T00:00:00Z')), 365);
 assert.equal(anniversaryDays({ ...a, recurrence: 'none' }, new Date('2025-02-28T01:00:00Z')), -365);
});
test('reading old messages stays distinct from being within one touch target of the end', () => {
 assert.equal(nearLatest(0, 1000, 300), false);
 assert.equal(nearLatest(670, 1000, 300), true);
 assert.equal(nearLatest(0, 200, 300), true);
});
