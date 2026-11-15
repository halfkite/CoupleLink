export type User = { id: string; display_name: string; username: string };
export type Couple = { id: string; partner: User; days_together: number; start_at: string; version: number; timezone?: string };
export type Anniversary = { id: string; title: string; date: string; recurrence: 'none' | 'yearly'; reminder_time: string; timezone: string; version: number; next_reminder: string | null };
export type Message = { id: string; couple_id: string; sender_id: string; body: string; seq: number; created_at: string; delivered: boolean; status?: 'queued' | 'failed'; failure?: string };
export type Session = { origin: string; token: string };
export type ChatCache = { scope: string; cursor: number; read: number; messages: Message[]; draft: string };
export interface Cache { load(scope: string): Promise<ChatCache>; save(value: ChatCache): Promise<void>; }
export interface Device {
  sessionRead(): Promise<Session | null>; sessionWrite(value: Session | null): Promise<void>;
  background(options: { enabled: boolean; origin: string; user: string; couple: string; cursor: number; sound: boolean; vibrate: boolean }): Promise<void>;
  notify(kind: string, title: string, body: string): Promise<void>;
  feedback(sound: boolean): Promise<string>;
  deliverInteraction?(options: { action: Action; title: string; body: string; allowSound: boolean; allowVibrate: boolean; expiresAt: number }): Promise<{ result: string; notificationError?: string }>;
}
export type Transport = (origin: string, path: string, method: string, body?: unknown, token?: string) => Promise<{ status: number; data: any }>;
export class ApiError extends Error { constructor(public code: string) { super(code); } }
export const actions = ['poke', 'miss_you', 'hug', 'vibrate', 'ring'] as const;
export type Action = typeof actions[number];
export function newID(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16)); bytes[6] = (bytes[6] & 15) | 64; bytes[8] = (bytes[8] & 63) | 128;
  const hex = [...bytes].map(n => n.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
export function normalizeOrigin(value: string): string {
  let url: URL;
  try { url = new URL(value.trim()); } catch { throw new ApiError('invalid_origin'); }
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash || url.pathname !== '/') throw new ApiError('invalid_origin');
  // Keep the same approved exact-host policy as the existing Android build.
  if (url.protocol === 'http:' && !['localhost', '127.0.0.1', '10.0.2.2', '10.74.205.253', '192.168.137.1'].includes(url.hostname)) throw new ApiError('http_not_enabled');
  return url.origin;
}
/** Accepts "http://host:port-keyxxxx": the key follows the last dash that splits into a valid origin. */
export function parseLoginLink(value: string): { origin: string; key: string } {
  const text = value.trim();
  for (let index = text.length - 1; index > 0; index--) {
    if (text[index] !== '-') continue;
    const key = text.slice(index + 1), origin = text.slice(0, index);
    if (!/^key[a-z0-9]{16,32}$/.test(key)) continue;
    try { return { origin: normalizeOrigin(origin), key }; } catch { /* keep scanning for an earlier split */ }
  }
  throw new ApiError('invalid_login_link');
}
export function validateMessage(m: any, couple: string): m is Message {
  return m && typeof m.id === 'string' && m.couple_id === couple && typeof m.sender_id === 'string' && typeof m.body === 'string' && Number.isSafeInteger(m.seq) && m.seq > 0 && typeof m.delivered === 'boolean' && typeof m.created_at === 'string';
}
export class Client {
  state = { origin: '', user: null as User | null, couple: null as Couple | null, messages: [] as Message[], draft: '', cursor: 0, unread: false, ready: false, online: false, busy: false, error: '', notificationError: '', notice: '', invitation: '', incoming: '' as Action | '', allowSound: true, allowVibrate: true, background: false };
  memories = { supported: false, anniversaries: [] as Anniversary[], results: [] as Message[], context: null as Message[] | null, query: '', before: 0, more: false };
  private searchEpoch = 0;
  private token = ''; private epoch = 0; private syncing = false; private cache: ChatCache | null = null; private chatVisible = false;
  constructor(private transport: Transport, private storage: Cache, private device: Device, private text: (key: string) => string) {}
  private scope() { return JSON.stringify([this.state.origin, this.state.user?.id, this.state.couple?.id]); }
  private valid(epoch: number) { return epoch === this.epoch; }
  private async request(path: string, method = 'GET', body?: unknown, auth = true) {
    const epoch = this.epoch, origin = this.state.origin, token = this.token;
    if (!origin || (auth && !token)) throw new ApiError('unauthorized');
    const result = await this.transport(origin, path, method, body, auth ? token : undefined);
    if (!this.valid(epoch)) throw new ApiError('stale_request');
    if (result.status < 200 || result.status >= 300) {
      const code = result.data?.code || 'server_error';
      if (auth && result.status === 401) await this.clearSession();
      throw new ApiError(code);
    }
    return result.data;
  }
  async test(origin: string) {
    const result = await this.transport(normalizeOrigin(origin), '/healthz', 'GET');
    if (result.status !== 200 || result.data?.api_version !== 1 || result.data?.status !== 'ok') throw new ApiError('incompatible_server');
    return result.data.version as string;
  }
  async restore() {
    const saved = await this.device.sessionRead();
    if (!saved) return;
    this.state.origin = normalizeOrigin(saved.origin); this.token = saved.token;
    await this.snapshot();
  }
  async changeServer(origin: string) {
    const next = normalizeOrigin(origin);
    await this.clearSession();
    this.state.origin = next;
  }
  private async stopBackground() {
    await this.device.background({ enabled: false, origin: '', user: '', couple: '', cursor: 0, sound: false, vibrate: false });
    this.state.background = false;
  }
  private async clearSession() {
    ++this.epoch; this.token = ''; this.cache = null;
    this.clearMemories();
    Object.assign(this.state, { user: null, couple: null, messages: [], draft: '', cursor: 0, unread: false, ready: false, incoming: '', invitation: '', online: false, notificationError: '' });
    await this.stopBackground();
    await this.device.sessionWrite(null);
  }
  async authenticate(username: string, password: string, displayName: string, register: boolean, deviceId: string) {
    if (password.length < 8 || password.length > 128) throw new ApiError('invalid_password');
    await this.test(this.state.origin);
    const data = await this.request(register ? '/auth/register' : '/auth/login', 'POST', {
      username: username.trim().toLowerCase(), password, device_id: deviceId, device_label: 'CoupleLink Web Android', platform: 'android', ...(register ? { display_name: displayName.trim() } : {}),
    }, false);
    if (typeof data.token !== 'string' || data.token.length !== 43) throw new ApiError('invalid_response');
    this.token = data.token;
    await this.device.sessionWrite({ origin: this.state.origin, token: this.token });
    await this.snapshot();
  }
  async authenticateKey(origin: string, key: string, deviceId: string) {
    await this.changeServer(origin);
    await this.test(this.state.origin);
    const data = await this.request('/auth/key', 'POST', { key, device_id: deviceId, device_label: 'CoupleLink Web Android', platform: 'android' }, false);
    if (typeof data.token !== 'string' || data.token.length !== 43) throw new ApiError('invalid_response');
    this.token = data.token;
    await this.device.sessionWrite({ origin: this.state.origin, token: this.token });
    await this.snapshot();
  }
  async logout() { try { await this.request('/auth/logout', 'POST'); } finally { await this.clearSession(); } }
  async snapshot() {
    let epoch = this.epoch;
    const data = await this.request('/account');
    if (!data.user || typeof data.user.id !== 'string' || (data.couple && (typeof data.couple.id !== 'string' || !data.couple.partner))) throw new ApiError('invalid_response');
    const previous = this.state.couple?.id;
    if (previous !== data.couple?.id) { epoch = ++this.epoch; this.clearMemories(); }
    this.memories.supported = Array.isArray(data.features) && data.features.includes('memories_v1');
    this.state.user = data.user; this.state.couple = data.couple || null;
    if (previous !== this.state.couple?.id || !this.cache) {
      if (previous) await this.stopBackground();
      if (!this.valid(epoch)) return;
      this.state.ready = false; this.state.messages = []; this.cache = null; this.state.unread = false; this.state.draft = ''; this.state.cursor = 0; this.state.incoming = ''; this.state.notice = '';
      if (this.state.couple) {
        const scope = this.scope(), saved = await this.storage.load(scope);
        if (!this.valid(epoch) || scope !== this.scope()) return;
        this.cache = saved; this.state.messages = saved.messages; this.state.draft = saved.draft; this.state.cursor = saved.cursor; this.state.ready = true;
        this.updateUnread();
      }
    }
  }
  private updateUnread() { this.state.unread = !!this.cache && this.cache.messages.some(m => m.sender_id !== this.state.user?.id && m.seq > this.cache!.read); }
  private async save() { if (this.cache) await this.storage.save(this.cache); }
  async setChatVisible(visible: boolean) {
    this.chatVisible = visible;
    if (visible && this.cache && !documentHidden()) { this.cache.read = this.cache.cursor; this.updateUnread(); await this.save(); }
  }
  async draft(value: string) { if (!this.cache) return; this.state.draft = value; this.cache.draft = value; await this.save(); }
  async send() {
    if (!this.cache || !this.state.couple || !this.state.user) throw new ApiError('not_bound');
    const body = this.state.draft;
    if (!body.trim() || [...body].length > 4000 || body.includes('\0')) throw new ApiError('invalid_message');
    const message: Message = { id: newID(), couple_id: this.state.couple.id, sender_id: this.state.user.id, body, seq: 0, created_at: new Date().toISOString(), delivered: false, status: 'queued' };
    this.cache.messages.push(message); this.cache.draft = ''; this.state.draft = ''; this.state.messages = this.cache.messages;
    await this.save(); // A send cannot reach the network before the queue is durable.
    await this.sync();
  }
  async sync() {
    if (this.syncing || !this.token) return;
    this.syncing = true; const epoch = this.epoch;
    try {
      await this.snapshot();
      if (!this.valid(epoch) || !this.cache || !this.state.couple) return;
      const cache = this.cache, couple = this.state.couple.id;
      await this.save();
      for (const queued of cache.messages.filter(m => m.status === 'queued')) {
        try {
          const data = await this.request('/messages', 'POST', { id: queued.id, couple_id: couple, body: queued.body });
          if (!validateMessage(data.message, couple)) throw new ApiError('invalid_response');
          Object.assign(queued, data.message, { status: undefined, failure: undefined }); await this.save();
        } catch (e) {
          if (!this.valid(epoch)) throw e;
          if (e instanceof ApiError && ['invalid_message', 'message_id_conflict'].includes(e.code)) { queued.status = 'failed'; queued.failure = e.code; await this.save(); }
          else throw e;
        }
      }
      // One page per poll keeps long histories from monopolising the UI.
      const data = await this.request(`/messages?couple_id=${couple}&after=${cache.cursor}`);
      if (!Array.isArray(data.messages) || !data.messages.every((m: any) => validateMessage(m, couple)) || !Number.isSafeInteger(data.next_cursor) || data.next_cursor < cache.cursor || (data.messages.length && data.messages[data.messages.length - 1].seq !== data.next_cursor)) throw new ApiError('invalid_response');
      const incoming = data.messages.some((m: Message) => m.sender_id !== this.state.user?.id && m.seq > cache.cursor);
      for (const message of data.messages as Message[]) {
        const index = cache.messages.findIndex(m => m.id === message.id);
        if (index < 0) cache.messages.push(message); else cache.messages[index] = message;
      }
      for (const m of cache.messages) if (m.sender_id === this.state.user?.id && m.seq > 0 && m.seq <= data.peer_delivered_through) m.delivered = true;
      cache.cursor = data.next_cursor;
      cache.messages.sort((a, b) => (a.seq || Number.MAX_SAFE_INTEGER) - (b.seq || Number.MAX_SAFE_INTEGER));
      if (this.chatVisible && !documentHidden()) cache.read = cache.cursor;
      await this.save();
      if (!this.valid(epoch)) return;
      this.state.messages = [...cache.messages]; this.state.cursor = cache.cursor; this.updateUnread();
      if (cache.cursor > 0) await this.request('/messages/receipts', 'POST', { couple_id: couple, through: cache.cursor });
      this.state.online = true; this.state.error = '';
      if (incoming && !this.chatVisible) await this.notify('messages', this.text('notificationTitle'), this.text('newMessage'));
      if (!documentHidden()) await this.claimInteraction(couple, epoch);
      if (this.memories.supported && !documentHidden()) await this.receiveReminders(couple);
    } catch (e) {
      if (this.valid(epoch)) { this.state.online = false; this.state.error = errorCode(e); }
    } finally { this.syncing = false; }
  }
  async invitation() { const data = await this.request('/couples/invitations', 'POST'); if (typeof data.token !== 'string') throw new ApiError('invalid_response'); this.state.invitation = data.token; }
  private clearMemories() {
    ++this.searchEpoch;
    Object.assign(this.memories, { supported: false, anniversaries: [], results: [], context: null, query: '', before: 0, more: false });
  }
  private memoryCouple() {
    if (!this.state.couple) throw new ApiError('not_bound');
    if (!this.memories.supported) throw new ApiError('upgrade_required');
    return this.state.couple;
  }
  async updateStartDate(date: string, version: number) {
    const couple = this.memoryCouple();
    await this.request('/couples/current', 'PUT', { couple_id: couple.id, start_at: date, version }); await this.snapshot();
  }
  async loadAnniversaries() {
    const couple = this.memoryCouple(); const data = await this.request(`/anniversaries?couple_id=${couple.id}`);
    if (!Array.isArray(data.anniversaries)) throw new ApiError('invalid_response');
    this.memories.anniversaries = data.anniversaries;
  }
  async saveAnniversary(value: { id: string; title: string; date: string; recurrence: 'none' | 'yearly'; reminder_time: string; version: number }, create: boolean) {
    const couple = this.memoryCouple();
    await this.request(create ? '/anniversaries' : `/anniversaries/${value.id}`, create ? 'POST' : 'PUT', { ...value, couple_id: couple.id });
    await this.loadAnniversaries();
  }
  async deleteAnniversary(id: string, version: number) {
    const couple = this.memoryCouple();
    await this.request(`/anniversaries/${id}`, 'DELETE', { couple_id: couple.id, version }); await this.loadAnniversaries();
  }
  async searchMessages(query: string, more = false) {
    const couple = this.memoryCouple(), generation = ++this.searchEpoch;
    const q = query.trim(); if (!q || [...q].length > 100) throw new ApiError('invalid_search');
    const append = more && this.memories.query === q, before = append ? this.memories.before : 0;
    const data = await this.request(`/messages/search?couple_id=${couple.id}&q=${encodeURIComponent(q)}${before ? '&before=' + before : ''}`);
    if (generation !== this.searchEpoch) return;
    if (!Array.isArray(data.messages) || !data.messages.every((m: any) => validateMessage(m, couple.id)) || !Number.isSafeInteger(data.next_before)) throw new ApiError('invalid_response');
    this.memories.results = append ? [...this.memories.results, ...data.messages] : data.messages;
    this.memories.query = q; this.memories.before = data.next_before; this.memories.more = data.has_more === true;
  }
  async openMessage(seq: number) {
    const couple = this.memoryCouple(), generation = ++this.searchEpoch;
    const data = await this.request(`/messages/context?couple_id=${couple.id}&seq=${seq}`);
    if (generation !== this.searchEpoch) return;
    if (!Array.isArray(data.messages) || !data.messages.every((m: any) => validateMessage(m, couple.id))) throw new ApiError('invalid_response');
    this.memories.context = data.messages;
  }
  closeHistory() { ++this.searchEpoch; this.memories.context = null; }
  private async receiveReminders(couple: string) {
    const epoch = this.epoch;
    const data = await this.request(`/anniversaries/reminders?couple_id=${couple}`);
    if (!Array.isArray(data.reminders)) throw new ApiError('invalid_response');
    for (const item of data.reminders) {
      if (typeof item.id !== 'string' || typeof item.title !== 'string') throw new ApiError('invalid_response');
      try { await this.device.notify('anniversary', this.text('anniversaryReminder'), item.title); }
      catch (e) { this.state.notificationError = errorCode(e); return; }
      if (!this.valid(epoch)) return;
      await this.request('/anniversaries/reminders/receipts', 'POST', { couple_id: couple, id: item.id });
      this.state.notice = 'anniversaryReminder';
    }
  }
  async join(token: string, date: string) {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) throw new ApiError('invalid_input');
    await this.request('/couples/join', 'POST', { token: token.trim(), start_at: date, timezone: 'Asia/Shanghai' }); await this.snapshot();
  }
  async unbind() {
    if (!this.state.couple) return;
    await this.request('/couples/current', 'DELETE', { version: this.state.couple.version });
    await this.snapshot();
  }
  async interact(action: Action) {
    if (!this.state.couple) throw new ApiError('not_bound');
    await this.request('/interactions', 'POST', { id: newID(), couple_id: this.state.couple.id, action });
    this.state.notice = 'interactionSent';
  }
  private async claimInteraction(couple: string, epoch: number) {
    if (this.state.incoming) return;
    const start = performance.now(), data = await this.request('/interactions/claim', 'POST', { couple_id: couple });
    if (!data.interaction) return;
    const { id, action } = data.interaction;
    if (!actions.includes(action) || typeof id !== 'string' || typeof data.remaining_ms !== 'number') throw new ApiError('invalid_response');
    const allowed = action === 'ring' ? this.state.allowSound : this.state.allowVibrate;
    const fresh = performance.now() - start < data.remaining_ms;
    let result = 'suppressed';
    if (!this.valid(epoch)) return;
    this.state.incoming = action;
    if (this.device.deliverInteraction) {
      try {
        const delivery = await this.device.deliverInteraction({ action, title: this.text('interactionTitle'), body: this.text(action), allowSound: this.state.allowSound, allowVibrate: this.state.allowVibrate, expiresAt: documentHidden() ? 0 : Date.now() + Math.max(0, data.remaining_ms - (performance.now() - start)) });
        if (!['requested', 'suppressed', 'unsupported', 'failed'].includes(delivery.result)) throw new ApiError('invalid_response');
        result = delivery.result;
        if (this.valid(epoch)) this.state.notificationError = delivery.notificationError || '';
      } catch (e) { result = 'failed'; if (this.valid(epoch)) this.state.notificationError = errorCode(e); }
    } else {
      if (allowed && fresh && !documentHidden()) result = await this.device.feedback(action === 'ring');
      await this.notify('interactions', this.text('interactionTitle'), this.text(action));
    }
    if (!this.valid(epoch)) return;
    await this.request('/interactions/result', 'POST', { id, couple_id: couple, result });
  }
  async background(enabled: boolean) {
    if (enabled && (!this.state.couple || !this.state.user || !this.cache)) throw new ApiError('not_bound');
    await this.device.background({ enabled, origin: this.state.origin, user: this.state.user?.id || '', couple: this.state.couple?.id || '', cursor: this.cache?.cursor || 0, sound: this.state.allowSound, vibrate: this.state.allowVibrate });
    this.state.background = enabled;
  }
  private async notify(kind: string, title: string, body: string) {
    const epoch = this.epoch;
    try { await this.device.notify(kind, title, body); if (this.valid(epoch)) this.state.notificationError = ''; }
    catch (e) { if (this.valid(epoch)) this.state.notificationError = errorCode(e); }
    // Notification failure is visible, but cannot turn a persisted message into
    // a network failure or prevent reporting an already executed interaction.
  }
}
function documentHidden() { return typeof document !== 'undefined' && document.hidden; }
export function errorCode(e: unknown) { return e instanceof ApiError ? e.code : e instanceof Error && e.message === 'storage_failed' ? 'storage_failed' : e && typeof e === 'object' && 'code' in e && typeof e.code === 'string' ? e.code : 'network_failed'; }
