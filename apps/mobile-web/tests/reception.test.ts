import test from 'node:test';
import assert from 'node:assert/strict';
import { AutoReception, type ReceptionDependencies } from '../src/reception.ts';
function fixture() {
  let scope = 'server/user/couple', foreground = true, permitted = true, running = false, prompted = false, permissionGrant = true, starts = 0, requests = 0, stops = 0;
  const deps: ReceptionDependencies = {
    context: () => ({ scope, foreground }), status: async () => ({ running, notificationsEnabled: permitted }),
    permission: async () => { requests++; return { granted: permissionGrant }; },
    start: async () => { starts++; running = true; }, stop: async () => { stops++; running = false; },
    wasPrompted: () => prompted, markPrompted: () => { prompted = true; }, update: value => { running = value; },
  };
  return { deps, auto: new AutoReception(deps), scope(v: string) { scope = v; }, foreground(v: boolean) { foreground = v; }, permitted(v: boolean) { permitted = v; }, grant(v: boolean) { permissionGrant = v; }, get starts() { return starts; }, get requests() { return requests; }, get stops() { return stops; } };
}
test('paired users with existing notification permission receive automatically without another switch', async () => {
  const f = fixture(); await f.auto.ensure(); await f.auto.ensure(); assert.equal(f.starts, 1); assert.equal(f.requests, 0);
});
test('one enable action obtains permission and starts reception', async () => {
  const f = fixture(); f.permitted(false); await f.auto.ensure(true); assert.equal(f.requests, 1); assert.equal(f.starts, 1);
});
test('permission refusal is respected without repeating prompts on every resume', async () => {
  const f = fixture(); f.permitted(false); f.grant(false); await f.auto.ensure(); await f.auto.ensure(); assert.equal(f.requests, 1); assert.equal(f.starts, 0);
});
test('unpaired or background pages cannot silently start a service or request permission', async () => {
  const f = fixture(); f.scope(''); await f.auto.ensure(); f.scope('bound'); f.foreground(false); await f.auto.ensure(); assert.equal(f.starts, 0); assert.equal(f.requests, 0);
});
test('a revoked system permission stops reception instead of being bypassed', async () => {
  const f = fixture(); await f.auto.ensure(); f.permitted(false); f.grant(false); await f.auto.ensure(); assert.equal(f.stops, 1); assert.equal(f.requests, 0);
});
test('switching account during a permission request cannot start reception for the old account', async () => {
  const f = fixture(); f.permitted(false); f.deps.permission = async () => { f.scope('another'); return { granted: true }; }; await f.auto.ensure(true); assert.equal(f.starts, 0);
});
