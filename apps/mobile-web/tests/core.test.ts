import test from 'node:test';
import assert from 'node:assert/strict';
import { ApiError, Client, normalizeOrigin, parseLoginLink, type Cache, type ChatCache, type Device, type Session, type Transport, type Message } from '../src/core.ts';
class MemoryCache implements Cache {
  values = new Map<string, ChatCache>(); fail = false;
  async load(scope: string) { return structuredClone(this.values.get(scope) || { scope, cursor: 0, read: 0, messages: [], draft: '' }); }
  async save(value: ChatCache) { if (this.fail) throw new Error('storage_failed'); this.values.set(value.scope, structuredClone(value)); }
}
function fixture() {
  const storage = new MemoryCache(), calls: { origin: string; path: string; token?: string; body?: any }[] = [], messages: Message[] = [];
  let session: Session | null = null, offline = false, revoked = false, notifications = 0, denyNotification = false, override: ((path: string) => Promise<any> | null) | null = null;
  const user = { id: 'user-a', display_name: 'A', username: 'aaa' }, couple = { id: 'couple-a', partner: { id: 'user-b', display_name: 'B', username: 'bbb' }, days_together: 365, start_at: '2025-10-01', version: 1 };
  const device: Device = { async sessionRead() { return session; }, async sessionWrite(value) { session = value; }, async background() {}, async feedback() { return 'requested'; }, async notify() { if (denyNotification) throw new ApiError('notification_denied'); notifications++; } };
  const transport: Transport = async (origin, path, method, body: any, token) => {
    calls.push({ origin, path, token, body });
    if (offline) throw new Error('offline');
    const replaced = override?.(path); if (replaced) return replaced;
    if (path === '/healthz') return { status: 200, data: { status: 'ok', api_version: 1, version: '1.0-r8' } };
    if (path === '/auth/login') return { status: 200, data: { token: 'x'.repeat(43) } };
    if (revoked) return { status: 401, data: { code: 'unauthorized' } };
    if (path === '/account') return { status: 200, data: { user, couple } };
    if (path === '/messages' && method === 'POST') {
      let message = messages.find(m => m.id === body.id);
      if (!message) { message = { ...body, seq: messages.length + 1, sender_id: user.id, created_at: new Date().toISOString(), delivered: false }; messages.push(message!); }
      return { status: 201, data: { message } };
    }
    if (path.startsWith('/messages?')) { const after = Number(new URL('https://test' + path).searchParams.get('after')); const page = messages.filter(m => m.seq > after); return { status: 200, data: { messages: page, next_cursor: page.at(-1)?.seq || after, peer_delivered_through: 0 } }; }
    if (path === '/interactions/claim') return { status: 200, data: { interaction: null } };
    if (path === '/messages/receipts') return { status: 200, data: { ok: true } };
    if (path === '/interactions/result') return { status: 200, data: { ok: true } };
    throw new Error('Unexpected API route: ' + path);
  };
  const make = () => new Client(transport, storage, device, key => key);
  const client = make(); client.state.origin = 'http://192.168.137.1:18080';
  return { client, make, storage, device, calls, messages, user, couple, async login() { await client.authenticate('aaa', '12345678', '', false, crypto.randomUUID()); }, offline(v: boolean) { offline = v; }, denyNotification() { denyNotification = true; }, revoke() { revoked = true; }, override(value: typeof override) { override = value; }, get notifications() { return notifications; }, get session() { return session; } };
}
test('system-controlled feedback is enabled by default including ringing', () => {
  const f = fixture();
  assert.equal(f.client.state.allowSound, true);
  assert.equal(f.client.state.allowVibrate, true);
});

test('origins reject embedded credentials, paths and unconfigured HTTP hosts', () => {
  assert.equal(normalizeOrigin(' http://192.168.137.1:18080/ '), 'http://192.168.137.1:18080');
  for (const url of ['https://u:p@example.com', 'https://example.com/api', 'https://example.com?token=x', 'javascript:alert(1)']) assert.throws(() => normalizeOrigin(url), ApiError);
assert.equal(normalizeOrigin('http://example.com'), 'http://example.com');
  assert.equal(normalizeOrigin('https://example.com'), 'https://example.com');
});
test('offline queued text survives client restart and replays the same ID once', async () => {
  const f = fixture(); await f.login(); f.offline(true); await f.client.draft('a durable message'); await f.client.send();
  const id = f.client.state.messages[0].id; assert.equal(f.client.state.messages[0].status, 'queued');
  f.offline(false); const restarted = f.make(); await restarted.restore(); await restarted.sync(); await restarted.sync();
  assert.equal(f.messages.length, 1); assert.equal(f.messages[0].id, id); assert.equal(restarted.state.messages[0].status, undefined);
});
test('storage failure cannot leak an undurable queued message to the network', async () => {
  const f = fixture(); await f.login(); await f.client.draft('save me first'); f.storage.fail = true;
  await assert.rejects(() => f.client.send(), /storage_failed/); await f.client.sync(); assert.equal(f.messages.length, 0);
  f.storage.fail = false; await f.client.sync(); assert.equal(f.messages.length, 1);
});
test('server switch fences delayed account responses and never forwards the old token', async () => {
  const f = fixture(); await f.login();
  let release!: (value: any) => void;
  f.override(path => path === '/account' ? new Promise(resolve => { release = resolve; }) : null);
  const pending = f.client.sync(); await new Promise(resolve => setTimeout(resolve, 0));
  await f.client.changeServer('https://other.example'); release({ status: 200, data: { user: f.user, couple: f.couple } }); await pending;
  assert.equal(f.client.state.user, null); assert.equal(f.client.state.messages.length, 0); assert.equal(f.session, null);
  f.override(null); await f.client.test('https://other.example');
  assert.ok(f.calls.filter(c => c.origin === 'https://other.example').every(c => !c.token));
});
test('unread only reflects partner messages and read marker survives restart', async () => {
  const f = fixture(); await f.login(); await f.client.draft('mine'); await f.client.send(); assert.equal(f.client.state.unread, false);
  f.messages.push({ ...f.messages[0], id: crypto.randomUUID(), sender_id: 'user-b', seq: 2, body: 'hello' });
  await f.client.sync(); assert.equal(f.client.state.unread, true); assert.equal(f.notifications, 1);
  await f.client.setChatVisible(true); assert.equal(f.client.state.unread, false);
  const restarted = f.make(); await restarted.restore(); assert.equal(restarted.state.unread, false);
});
test('malformed sync response does not advance cursor or acknowledge unpersisted messages', async () => {
  const f = fixture(); await f.login(); f.override(path => path.startsWith('/messages?') ? Promise.resolve({ status: 200, data: { messages: [{ couple_id: 'another-couple' }], next_cursor: 100 } }) : null);
  await f.client.sync(); assert.equal(f.client.state.cursor, 0); assert.equal(f.client.state.error, 'invalid_response'); assert.ok(!f.calls.some(c => c.path === '/messages/receipts'));
});
test('revoked session immediately hides shared content and removes stored credential', async () => {
  const f = fixture(); await f.login(); f.revoke(); await f.client.sync(); assert.equal(f.client.state.user, null); assert.equal(f.client.state.ready, false); assert.equal(f.session, null);
});
test('denied notification remains visible without misreporting network status or losing receipts', async () => {
  const f = fixture(); await f.login(); f.denyNotification();
  f.messages.push({ id: crypto.randomUUID(), couple_id: f.couple.id, sender_id: 'user-b', seq: 1, body: 'hello', created_at: new Date().toISOString(), delivered: false });
  await f.client.sync(); assert.equal(f.client.state.online, true); assert.equal(f.client.state.cursor, 1); assert.equal(f.client.state.notificationError, 'notification_denied'); assert.ok(f.calls.some(c => c.path === '/messages/receipts'));
});
test('native interaction delivery is one operation and reports its actual outcome', async () => {
  const f = fixture(); await f.login(); let deliveries = 0, directFeedback = 0;
  f.device.feedback = async () => { directFeedback++; return 'requested'; };
  f.device.deliverInteraction = async options => { deliveries++; assert.equal(options.action, 'vibrate'); assert.equal(options.allowVibrate, false); assert.equal(options.allowSound, false); assert.ok(options.expiresAt > Date.now()); return { result: 'suppressed' }; };
  f.override(path => path === '/interactions/claim' ? Promise.resolve({ status: 200, data: { interaction: { id: 'event-1', action: 'vibrate' }, remaining_ms: 10000 } }) : null);
  await f.client.sync(); await f.client.sync();
  assert.equal(deliveries, 1); assert.equal(directFeedback, 1); assert.equal(f.notifications, 0);
  assert.equal(f.calls.find(c => c.path === '/interactions/result')?.body.result, 'requested');
  assert.equal(f.client.state.incoming, 'vibrate');
});
test('blocked category is visible while interaction acknowledgement still completes', async () => {
  const f = fixture(); await f.login();
  f.device.deliverInteraction = async () => ({ result: 'suppressed', notificationError: 'channel_blocked' });

  f.device.feedback = async () => 'suppressed';
  f.override(path => path === '/interactions/claim' ? Promise.resolve({ status: 200, data: { interaction: { id: 'event-2', action: 'poke' }, remaining_ms: 10000 } }) : null);
  await f.client.sync(); assert.equal(f.client.state.notificationError, 'channel_blocked'); assert.equal(f.client.state.incoming, 'poke'); assert.equal(f.calls.find(c => c.path === '/interactions/result')?.body.result, 'suppressed');
});
test('native failure is acknowledged as failed rather than pretending the motor ran', async () => {
  const f = fixture(); await f.login();
  f.device.deliverInteraction = async () => { throw new ApiError('feedback_failed'); };

  f.device.feedback = async () => 'suppressed';
  f.override(path => path === '/interactions/claim' ? Promise.resolve({ status: 200, data: { interaction: { id: 'event-3', action: 'hug' }, remaining_ms: 10000 } }) : null);
  await f.client.sync(); assert.equal(f.client.state.notificationError, 'feedback_failed'); assert.equal(f.calls.find(c => c.path === '/interactions/result')?.body.result, 'failed');
});

test('search and historical context never move the durable sync cursor', async () => {
  const f = fixture(); await f.login(); f.client.memories.supported = true;
  const m = { id: crypto.randomUUID(), couple_id: f.couple.id, sender_id: 'user-b', body: 'old message', seq: 900, created_at: new Date().toISOString(), delivered: false };
  f.override(path => path.startsWith('/messages/search?') ? Promise.resolve({ status: 200, data: { messages: [m], next_before: 900, has_more: false } }) : path.startsWith('/messages/context?') ? Promise.resolve({ status: 200, data: { messages: [m] } }) : null);
  await f.client.searchMessages('old'); await f.client.openMessage(900);
  assert.equal(f.client.memories.results.length, 1); assert.equal(f.client.memories.context?.[0].seq, 900);
  assert.equal(f.client.state.cursor, 0); assert.equal(f.client.state.messages.length, 0);
  assert.equal(f.calls.filter(c => c.path === '/messages/receipts').length, 0);
  await f.client.changeServer('https://other.example');
  assert.equal(f.client.memories.results.length, 0); assert.equal(f.client.memories.context, null);
});
test('a delayed older search cannot replace the latest query', async () => {
  const f = fixture(); await f.login(); f.client.memories.supported = true;
  let release!: (value: any) => void;
  f.override(path => path.includes('q=old') ? new Promise(resolve => { release = resolve; }) : path.includes('q=new') ? Promise.resolve({ status: 200, data: { messages: [], next_before: 0, has_more: false } }) : null);
  const pending = f.client.searchMessages('old'); await new Promise(resolve => setTimeout(resolve, 0));
  await f.client.searchMessages('new'); release({ status: 200, data: { messages: [], next_before: 0, has_more: false } }); await pending;
  assert.equal(f.client.memories.query, 'new');
});
test('reminder acknowledgement waits for successful native notification', async () => {
  const f = fixture(); await f.login(); f.denyNotification();
  f.override(path => path === '/account' ? Promise.resolve({ status: 200, data: { user: f.user, couple: f.couple, features: ['memories_v1'] } }) : path.startsWith('/anniversaries/reminders?') ? Promise.resolve({ status: 200, data: { reminders: [{ id: crypto.randomUUID(), title: 'special day' }] } }) : null);
  await f.client.sync(); assert.equal(f.client.state.notificationError, 'notification_denied');
  assert.equal(f.calls.some(c => c.path === '/anniversaries/reminders/receipts'), false);
});

test('login links carry the server origin and the group key', () => {
  assert.deepEqual(parseLoginLink('http://127.0.0.1:30710-keyabcdefghijklmnopqrst'), { origin: 'http://127.0.0.1:30710', key: 'keyabcdefghijklmnopqrst' });
  assert.deepEqual(parseLoginLink(' http://192.168.137.1:30710-key234567abcdefghijklmn '), { origin: 'http://192.168.137.1:30710', key: 'key234567abcdefghijklmn' });
  assert.throws(() => parseLoginLink('http://127.0.0.1:30710'), (error: any) => error.code === 'invalid_login_link');
  assert.deepEqual(parseLoginLink('http://8.8.8.8:30710-keyabcdefghijklmnopqrst'), { origin: 'http://8.8.8.8:30710', key: 'keyabcdefghijklmnopqrst' });
});