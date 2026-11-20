export type User = { id: string; display_name: string; username: string; gender?: string };

export type Couple = { id: string; partner: User; days_together: number; start_at: string; version: number; timezone?: string; my_remark?: string; partner_remark?: string; my_avatar?: string; partner_avatar?: string };

export type Anniversary = { id: string; title: string; date: string; recurrence: 'none' | 'yearly'; reminder_time: string; timezone: string; version: number; next_reminder: string | null };

export type Message = { id: string; couple_id: string; sender_id: string; body: string; seq: number; created_at: string; delivered: boolean; recalled_at?: string | null; image_id?: string | null; status?: 'queued' | 'failed'; failure?: string };

export type Note = { id: string; couple_id: string; author_id: string; title: string; body: string; image_id: string | null; featured: boolean; created_at: string; updated_at: string };
export type Session = { origin: string; token: string };

export type ChatCache = { scope: string; cursor: number; read: number; messages: Message[]; draft: string; notes?: Note[] };

export type CareProfile = { timezone: string; average_cycle_days: number; average_period_days: number; reminder_enabled: boolean };

export type PeriodRecord = { id: string; start_date: string; end_date: string | null; flow_level: 'light' | 'medium' | 'heavy'; note: string; created_at: string; updated_at: string };

export type CycleLog = { id: string; log_date: string; pain_level: number; mood: string; symptoms: string[]; note: string; created_at?: string; updated_at?: string };

export type CarePreferences = { preferred_support: string[]; avoid_support: string[]; note: string };

export type CareScope = 'cycle_summary' | 'period_dates' | 'daily_logs' | 'care_preferences';

export type CareShare = { id: string; scope: CareScope; granted_at: string };

export type CareTask = { id: string; title: string; note: string; due_at: string | null; status: 'open' | 'done'; created_by_me: boolean; subject_me: boolean; created_at: string; updated_at: string; completed_at?: string | null };

export type CareCycleEntry = { start_date: string; end_date: string | null; length_days: number | null };

export type CareStatistics = { average_cycle_days: number; average_period_days: number; cycles: CareCycleEntry[]; predicted_start_date: string | null; predicted_end_date: string | null; days_until_next: number | null; confidence: 'estimate' };

export type CareReminder = { id: string; kind: 'period_upcoming' | 'period_start' | 'log_nudge' | 'care_task'; scheduled_at: string };

/** The partner endpoint never sends flow_level/note: only the shared start/end dates. */
export type PartnerPeriod = { id: string; start_date: string; end_date: string | null };

export type PartnerCare = { display_name: string; grants: Record<CareScope, boolean>; summary: { predicted_start_date: string | null; days_until_next: number | null; estimate: boolean } | null; periods: PartnerPeriod[] | null; logs: CycleLog[] | null; preferences: CarePreferences | null };

export type CareDashboard = { profile: { average_cycle_days: number; average_period_days: number; reminder_enabled: boolean }; today: { in_period: boolean; period_day: number; has_log: boolean; open_tasks: number }; prediction: { start_date: string | null; days_until: number | null; estimate: boolean } | null; partner: { display_name: string; grants: Record<CareScope, boolean>; summary: PartnerCare['summary'] } | null; shares: { active: CareScope[]; available: CareScope[] } };

function isCareObject(value: unknown): value is Record<string, any> { return !!value && typeof value === 'object' && !Array.isArray(value); }

const CARE_SCOPES: CareScope[] = ['cycle_summary', 'period_dates', 'daily_logs', 'care_preferences'];

const CARE_REMINDER_KINDS: Array<CareReminder['kind']> = ['period_upcoming', 'period_start', 'log_nudge', 'care_task'];

function careProfileValue(value: unknown): CareProfile {

  if (!isCareObject(value) || typeof value.timezone !== 'string' || typeof value.average_cycle_days !== 'number' || typeof value.average_period_days !== 'number' || typeof value.reminder_enabled !== 'boolean') throw new ApiError('invalid_response');

  return value as CareProfile;

}

function carePeriodValue(value: unknown): PeriodRecord {

  if (!isCareObject(value) || typeof value.id !== 'string' || typeof value.start_date !== 'string') throw new ApiError('invalid_response');

  return value as PeriodRecord;

}

function careLogValue(value: unknown): CycleLog {

  if (!isCareObject(value) || typeof value.id !== 'string' || typeof value.log_date !== 'string') throw new ApiError('invalid_response');

  return value as CycleLog;

}

function carePreferencesValue(value: unknown): CarePreferences {

  if (!isCareObject(value) || !Array.isArray(value.preferred_support) || !Array.isArray(value.avoid_support) || typeof value.note !== 'string') throw new ApiError('invalid_response');

  return value as CarePreferences;

}

function careShareValue(value: unknown): CareShare {

  if (!isCareObject(value) || typeof value.id !== 'string' || !CARE_SCOPES.includes(value.scope)) throw new ApiError('invalid_response');

  return value as CareShare;

}

function careTaskValue(value: unknown): CareTask {

  if (!isCareObject(value) || typeof value.id !== 'string' || typeof value.title !== 'string') throw new ApiError('invalid_response');

  return value as CareTask;

}

function careReminderValue(value: unknown): CareReminder {

  if (!isCareObject(value) || typeof value.id !== 'string' || !CARE_REMINDER_KINDS.includes(value.kind)) throw new ApiError('invalid_response');

  return value as CareReminder;

}

export interface Cache {
  load(scope: string): Promise<ChatCache>;
  save(value: ChatCache): Promise<void>;
  loadSession?(origin: string): Promise<{ user: unknown; couple: unknown } | null>;
  saveSession?(origin: string, user: unknown, couple: unknown): Promise<void>;
  loadNotes?(scope: string): Promise<unknown[] | null>;
  saveNotes?(scope: string, notes: unknown[]): Promise<void>;
  loadImage?(id: string): Promise<Blob | null>;
  saveImage?(id: string, blob: Blob): Promise<void>;
}

export interface Device {

  sessionRead(): Promise<Session | null>; sessionWrite(value: Session | null): Promise<void>;

  background(options: { enabled: boolean; origin: string; user: string; couple: string; cursor: number; sound: boolean; vibrate: boolean }): Promise<void>;

  notify(kind: string, title: string, body: string): Promise<void>;

  feedback(sound: boolean): Promise<string>;

  deliverInteraction?(options: { action: Action; title: string; body: string; allowSound: boolean; allowVibrate: boolean; expiresAt: number }): Promise<{ result: string; notificationError?: string }>;
  openUrl?(url: string): Promise<void>;
  downloadAndInstall?(url: string, sha256?: string): Promise<void>;

  fetchImage?(origin: string, id: string, token: string): Promise<{ contentType: string; data: string } | null>;

}

export type Transport = (origin: string, path: string, method: string, body?: unknown, token?: string) => Promise<{ status: number; data: any }>;

export class ApiError extends Error { constructor(public code: string) { super(code); } }

export const APP_VERSION = '1.0-r63';

/** Orders release revisions so an update check cannot offer a downgrade. */

function revisionOf(value: string) { const match = /^1\.0-r(\d+)$/.exec(value.trim()); return match ? Number(match[1]) : -1; }

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

  // Self-hosted servers may also sit on a public address; the client only talks to

  // the origin the user typed, and the app still warns when that origin is HTTP.

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

  state = { origin: '', user: null as User | null, couple: null as Couple | null, messages: [] as Message[], draft: '', cursor: 0, peerRead: 0, unread: false, ready: false, online: false, busy: false, error: '', notificationError: '', notice: '', invitation: '', incoming: '' as Action | '', allowSound: true, allowVibrate: true, background: false, offline: false, images: {} as Record<string, string> };

  memories = { supported: false, anniversaries: [] as Anniversary[], results: [] as Message[], context: null as Message[] | null, query: '', before: 0, more: false };

  care = { supported: false, dashboard: null as CareDashboard | null, profile: null as CareProfile | null, periods: [] as PeriodRecord[], logs: [] as CycleLog[], preferences: null as CarePreferences | null, shares: [] as CareShare[], partner: null as PartnerCare | null, tasks: [] as CareTask[], statistics: null as CareStatistics | null, reminders: [] as CareReminder[] };

  private searchEpoch = 0;

  private token = ''; private epoch = 0; private syncing = false; private readSent = 0; private cache: ChatCache | null = null; private chatVisible = false;

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

    try {
      await this.snapshot();
    } catch (error) {
      // Offline start: show the locally cached chat instead of an empty login screen.
      const local = await this.storage.loadSession?.(this.state.origin);
      if (!local || !local.couple || !local.user) throw error;
      this.state.user = local.user as User;
      this.state.couple = JSON.parse(JSON.stringify(local.couple)) as Couple;
      const scope = this.scope(), cache = await this.storage.load(scope);
      this.cache = cache; this.state.messages = cache.messages; this.state.cursor = cache.cursor;
      this.state.draft = cache.draft; this.state.ready = true; this.state.offline = true;
      this.updateUnread();
      this.prefetchImages(cache.messages.map(message => message.image_id));
    }

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

    this.clearCare();

    Object.assign(this.state, { user: null, couple: null, messages: [], draft: '', cursor: 0, peerRead: 0, unread: false, ready: false, incoming: '', invitation: '', online: false, notificationError: '' });

    this.readSent = 0;

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

  async authenticateKey(origin: string, key: string, deviceId: string, gender = '') {

    await this.changeServer(origin);

    await this.test(this.state.origin);

    const data = await this.request('/auth/key', 'POST', { key, device_id: deviceId, device_label: 'CoupleLink Web Android', platform: 'android', ...(gender ? { gender } : {}) }, false);

    if (typeof data.token !== 'string' || data.token.length !== 43) throw new ApiError('invalid_response');

    this.token = data.token;

    await this.device.sessionWrite({ origin: this.state.origin, token: this.token });

    await this.snapshot();

  }

  async logout() { try { await this.request('/auth/logout', 'POST'); } finally { await this.clearSession(); } }

  /** Saves the caller's private label for their partner; both sides may read it. */

  async setRemark(remark: string) {

    const data = await this.request('/me/remark', 'PUT', { remark });

    if (data && data.couple) this.state.couple = data.couple;

    else { try { await this.snapshot(); } catch { /* saved; the next sync refreshes */ } }

  }

  async snapshot() {

    let epoch = this.epoch;

    const data = await this.request('/account');

    if (!data.user || typeof data.user.id !== 'string' || (data.couple && (typeof data.couple.id !== 'string' || !data.couple.partner))) throw new ApiError('invalid_response');

    const previous = this.state.couple?.id;

    if (previous !== data.couple?.id) { epoch = ++this.epoch; this.clearMemories(); }

    this.memories.supported = Array.isArray(data.features) && data.features.includes('memories_v1');

    // Health data belongs to one relationship: a changed or ended couple must
    // drop the cached partner view instead of leaving it visible until logout.
    const careSupported = Array.isArray(data.features) && data.features.includes('cycle_care_v1');
    if (previous !== data.couple?.id) this.clearCare();
    this.care.supported = careSupported;

    this.state.user = data.user; this.state.couple = data.couple || null; this.state.offline = false;
    if (data.couple) this.prefetchImages([data.couple.my_avatar, data.couple.partner_avatar]);
    if (this.state.user && this.state.couple) void this.storage.saveSession?.(this.state.origin, JSON.parse(JSON.stringify(this.state.user)), JSON.parse(JSON.stringify(this.state.couple)));

    if (previous !== this.state.couple?.id || !this.cache) {

      if (previous) await this.stopBackground();

      if (!this.valid(epoch)) return;

      this.state.ready = false; this.state.messages = []; this.cache = null; this.state.unread = false; this.state.draft = ''; this.state.cursor = 0; this.state.peerRead = 0; this.readSent = 0; this.state.incoming = ''; this.state.notice = '';

      if (this.state.couple) {

        const scope = this.scope(), saved = await this.storage.load(scope);

        if (!this.valid(epoch) || scope !== this.scope()) return;

        this.cache = saved; this.state.messages = saved.messages; this.state.draft = saved.draft; this.state.cursor = saved.cursor; this.state.ready = true;

        this.prefetchImages(saved.messages.map(message => message.image_id));

        this.updateUnread();

      }

    }

  }

  private updateUnread() { this.state.unread = !!this.cache && this.cache.messages.some(m => m.sender_id !== this.state.user?.id && m.seq > this.cache!.read); }

  private async save() { if (this.cache) await this.storage.save(this.cache); }

  /** Best-effort read receipt; a failure must not break syncing. */

  private async sendRead(couple: string, through: number) {

    if (through <= 0) return;

    try { await this.request('/messages/read', 'POST', { couple_id: couple, through }); this.readSent = through; } catch { /* retried on the next sync */ }

  }

  async setChatVisible(visible: boolean) {

    this.chatVisible = visible;

    if (visible && this.cache && !documentHidden()) {

      this.cache.read = this.cache.cursor; this.updateUnread(); await this.save();

      if (this.state.couple) await this.sendRead(this.state.couple.id, this.cache.read);

    }

  }

  async draft(value: string) { if (!this.cache) return; this.state.draft = value; this.cache.draft = value; await this.save(); }

  async send() { await this.queueMessage(this.state.draft, ''); }
  /** Queues a text or image message; the queue is durable before the network is touched. */
  async queueMessage(body: string, imageID: string) {
    if (!this.cache || !this.state.couple || !this.state.user) throw new ApiError('not_bound');
    if ((!body.trim() && !imageID) || [...body].length > 4000 || body.includes('\0')) throw new ApiError('invalid_message');
    const message: Message = { id: newID(), couple_id: this.state.couple.id, sender_id: this.state.user.id, body, image_id: imageID || null, seq: 0, created_at: new Date().toISOString(), delivered: false, status: 'queued' };
    this.cache.messages.push(message); this.cache.draft = ''; this.state.draft = ''; this.state.messages = this.cache.messages.slice();
    await this.save(); // A send cannot reach the network before the queue is durable.
    await this.sync();
  }
  /** Uploads a base64 image and returns the id used by messages and notes. */
  async uploadImage(base64: string): Promise<string> {
    if (!this.state.couple) throw new ApiError('not_bound');
    const data = await this.request('/images', 'POST', { couple_id: this.state.couple.id, data: base64 });
    if (!data || typeof data.id !== 'string') throw new ApiError('invalid_response');
    return data.id as string;
  }
  /** <img> tags cannot send headers, so the session token travels in the query. */
  /** Network URL for an image; used as a fallback until the blob is cached. */
  private networkImageURL(id: string) { return this.state.origin + '/images/' + id + '?token=' + encodeURIComponent(this.token); }

  /** Only a cached copy is exposed: WebView http loads are blocked for LAN origins. */
  imageURL(id: string) { return this.state.images[id] || ''; }

  /** Downloads a photo once into IndexedDB and exposes it as a blob URL. */
  async cacheImage(id: string) {
    if (!id || this.state.images[id] || typeof indexedDB === 'undefined') return;
    try {
      const cached = await this.storage.loadImage?.(id);
      if (cached) { this.state.images[id] = URL.createObjectURL(cached); return; }
      if (this.device.fetchImage) {
        // The Android WebView cannot reach the LAN origin for <img>, so the
        // native side downloads the bytes and we keep them as a data URL.
        const photo = await this.device.fetchImage(this.state.origin, id, this.token);
        if (!photo?.data) return;
        const url = 'data:' + (photo.contentType || 'image/jpeg') + ';base64,' + photo.data;
        this.state.images[id] = url;
        const blob = await (await fetch(url)).blob();
        await this.storage.saveImage?.(id, blob);
        return;
      }
      const response = await fetch(this.networkImageURL(id));
      if (!response.ok) return;
      const blob = await response.blob();
      if (!blob.size) return;
      await this.storage.saveImage?.(id, blob);
      this.state.images[id] = URL.createObjectURL(blob);
    } catch { /* stay without a photo */ }
  }

  /** Warms the cache for every photo referenced by the current view. */
  prefetchImages(ids: Array<string | null | undefined>) {
    const unique = [...new Set(ids.filter((id): id is string => !!id))].filter(id => !this.state.images[id]);
    void (async () => {
      for (const id of unique.slice(0, 40)) {
        if (!this.valid(this.epoch)) return;
        await this.cacheImage(id);
      }
    })();
  }
  async listNotes(): Promise<Note[]> {
    if (!this.state.couple) throw new ApiError('not_bound');
    try {
      const data = await this.request('/notes?couple_id=' + this.state.couple.id, 'GET');
      if (!data || !Array.isArray(data.notes)) throw new ApiError('invalid_response');
      const notes = data.notes as Note[];
      this.prefetchImages(notes.map(note => note.image_id));
      if (this.cache) { this.cache.notes = notes; await this.save(); }
      try { localStorage.setItem('cl.notes.' + this.state.couple.id, JSON.stringify(notes)); } catch { /* quota or private mode */ }
      return notes;
    } catch (error) {
      // Offline (or a server hiccup): fall back to the notes saved with the chat cache.
      let cached = this.cache?.notes;
      if (!cached || !cached.length) {
        try { const raw = localStorage.getItem('cl.notes.' + this.state.couple.id); if (raw) cached = JSON.parse(raw) as Note[]; } catch { /* ignore */ }
      }
      if (cached && cached.length) {
        this.prefetchImages(cached.map(note => note.image_id));
        return cached;
      }
      throw error;
    }
  }
  async setNoteFeatured(id: string, featured: boolean): Promise<Note> {
    if (!this.state.couple) throw new ApiError('not_bound');
    const data = await this.request('/notes/' + id + '/featured', 'POST', { couple_id: this.state.couple.id, featured });
    if (!data || !data.note) throw new ApiError('invalid_response');
    return data.note as Note;
  }  async saveNote(input: { id?: string; title: string; body: string; imageID: string }): Promise<Note> {
    if (!this.state.couple) throw new ApiError('not_bound');
    const payload = { couple_id: this.state.couple.id, title: input.title, body: input.body, image_id: input.imageID };
    const data = input.id ? await this.request('/notes/' + input.id, 'PUT', payload) : await this.request('/notes', 'POST', payload);
    if (!data || !data.note) throw new ApiError('invalid_response');
    return data.note as Note;
  }
  /** Points this member's avatar at an already uploaded photo (empty clears it). */
  async updateAvatar(imageID: string) {
    const data = await this.request('/me/avatar', 'PUT', { image_id: imageID });
    if (!data || typeof data.avatar_image_id !== 'string') throw new ApiError('invalid_response');
    if (this.state.couple) this.state.couple.my_avatar = data.avatar_image_id;
    if (data.avatar_image_id) void this.cacheImage(data.avatar_image_id);
    return data.avatar_image_id as string;
  }  /** Sets the nickname shown to the partner (and to this device). */
  async updateProfile(displayName: string) {
    const name = displayName.trim();
    if (!name || [...name].length > 24) throw new ApiError('invalid_input');
    const data = await this.request('/me/profile', 'PUT', { display_name: name });
    if (!data || !data.user || typeof data.user.display_name !== 'string') throw new ApiError('invalid_response');
    if (this.state.user) this.state.user.display_name = data.user.display_name;
    return data.user.display_name as string;
  }  async deleteNote(id: string): Promise<void> {
    if (!this.state.couple) throw new ApiError('not_bound');
    await this.request('/notes/' + id + '?couple_id=' + this.state.couple.id, 'DELETE');
  }

  /** Asks the server which packaged client it hosts; null means already current. */

  async checkUpdate(): Promise<{ version: string; url: string; size: number; sha256: string } | null> {

    const data = await this.request('/update', 'GET');

    if (!data || typeof data.version !== 'string' || typeof data.path !== 'string') throw new ApiError('invalid_response');

    if (revisionOf(data.version) <= revisionOf(APP_VERSION)) return null;

    return { version: data.version, url: this.state.origin + data.path, size: Number(data.size) || 0, sha256: typeof data.sha256 === 'string' ? data.sha256 : '' };

  }

  /** Hides a message for both members; only the sender can do it in the first three minutes. */

  async recall(id: string) {

    if (!this.cache || !this.state.couple) throw new ApiError('not_bound');

    const data = await this.request(`/messages/${id}/recall`, 'POST', { couple_id: this.state.couple.id });

    const at = typeof data.recalled_at === 'string' ? data.recalled_at : new Date().toISOString();

    const index = this.cache.messages.findIndex(m => m.id === id);

    if (index >= 0) { this.cache.messages[index] = { ...this.cache.messages[index], recalled_at: at }; await this.save(); }

    this.state.messages = this.cache.messages === this.state.messages ? this.cache.messages.slice() : this.cache.messages;

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

          const data = await this.request('/messages', 'POST', { id: queued.id, couple_id: couple, body: queued.body, image_id: queued.image_id || '' });

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

      if (Number.isSafeInteger(data.peer_read_through)) this.state.peerRead = data.peer_read_through as number;

      cache.cursor = data.next_cursor;

      cache.messages.sort((a, b) => (a.seq || Number.MAX_SAFE_INTEGER) - (b.seq || Number.MAX_SAFE_INTEGER));

      if (this.chatVisible && !documentHidden()) cache.read = cache.cursor;

      await this.save();

      if (!this.valid(epoch)) return;

      this.state.messages = [...cache.messages]; this.state.cursor = cache.cursor; this.updateUnread(); this.prefetchImages(this.state.messages.map(message => message.image_id)); await this.save();

      if (cache.cursor > 0) await this.request('/messages/receipts', 'POST', { couple_id: couple, through: cache.cursor });

      if (this.chatVisible && !documentHidden() && cache.read > this.readSent) await this.sendRead(couple, cache.read);

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

    // The start date is a basic couple field: no memories capability required.
    if (!this.state.couple) throw new ApiError('not_bound');
    const couple = this.state.couple;

    // Another device may have bumped the version; refresh before writing and
    // retry once so a stale tab still saves.
    let current = version;
    try { const fresh = await this.request('/couples/current', 'GET'); if (fresh?.couple && typeof fresh.couple.version === 'number') current = fresh.couple.version; } catch { /* keep the caller version */ }
    try {
      await this.request('/couples/current', 'PUT', { couple_id: couple.id, start_at: date, version: current });
    } catch (error) {
      if (!(error instanceof ApiError) || error.code !== 'version_conflict') throw error;
      const fresh = await this.request('/couples/current', 'GET');
      await this.request('/couples/current', 'PUT', { couple_id: couple.id, start_at: date, version: fresh.couple.version });
    }
    await this.snapshot();

  }

  async loadAnniversaries() {

    // The start date is a basic couple field: no memories capability required.
    if (!this.state.couple) throw new ApiError('not_bound');
    const couple = this.state.couple; const data = await this.request(`/anniversaries?couple_id=${couple.id}`);

    if (!Array.isArray(data.anniversaries)) throw new ApiError('invalid_response');

    this.memories.anniversaries = data.anniversaries;

  }

  async saveAnniversary(value: { id: string; title: string; date: string; recurrence: 'none' | 'yearly'; reminder_time: string; version: number }, create: boolean) {

    // The start date is a basic couple field: no memories capability required.
    if (!this.state.couple) throw new ApiError('not_bound');
    const couple = this.state.couple;

    await this.request(create ? '/anniversaries' : `/anniversaries/${value.id}`, create ? 'POST' : 'PUT', { ...value, couple_id: couple.id });

    await this.loadAnniversaries();

  }

  async deleteAnniversary(id: string, version: number) {

    // The start date is a basic couple field: no memories capability required.
    if (!this.state.couple) throw new ApiError('not_bound');
    const couple = this.state.couple;

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

  /** Resets every care view; the capability flag stays off until the next snapshot. */

  clearCare() {

    Object.assign(this.care, { supported: false, dashboard: null, profile: null, periods: [], logs: [], preferences: null, shares: [], partner: null, tasks: [], statistics: null, reminders: [] });

  }

  /** Refuses every care call on a server without cycle_care_v1 instead of leaking 404s. */

  private async careRequest(path: string, method = 'GET', body?: unknown): Promise<any> {

    if (!this.care.supported) throw new ApiError('upgrade_required');

    return this.request(path, method, body);

  }

  private careUpsert<T extends { id: string }>(list: T[], value: T) {

    const index = list.findIndex(entry => entry.id === value.id);

    if (index >= 0) list[index] = value; else list.push(value);

  }

  async loadCareDashboard(): Promise<CareDashboard> {

    const data = await this.careRequest('/care/dashboard');

    const dashboard = data?.dashboard;

    if (!isCareObject(dashboard) || !isCareObject(dashboard.today) || !isCareObject(dashboard.shares)) throw new ApiError('invalid_response');

    this.care.dashboard = dashboard as CareDashboard;

    return this.care.dashboard;

  }

  async loadCareProfile(): Promise<CareProfile> {

    const data = await this.careRequest('/care/profile');

    this.care.profile = careProfileValue(data?.profile);

    return this.care.profile;

  }

  async saveCareProfile(input: { timezone?: string; average_cycle_days?: number; average_period_days?: number; reminder_enabled?: boolean }): Promise<CareProfile> {

    const data = await this.careRequest('/care/profile', 'PUT', input);

    this.care.profile = careProfileValue(data?.profile);

    return this.care.profile;

  }

  async loadPeriods(from?: string, to?: string): Promise<PeriodRecord[]> {

    const query = new URLSearchParams();

    if (from) query.set('from', from);

    if (to) query.set('to', to);

    const suffix = query.toString();

    const data = await this.careRequest('/care/periods' + (suffix ? '?' + suffix : ''));

    if (!Array.isArray(data?.periods)) throw new ApiError('invalid_response');

    this.care.periods = data.periods.map(carePeriodValue);

    return this.care.periods;

  }

  async savePeriod(input: { id?: string; start_date: string; end_date?: string | null; flow_level?: string; note?: string }): Promise<PeriodRecord> {

    const data = await this.careRequest(input.id ? '/care/periods/' + input.id : '/care/periods', input.id ? 'PUT' : 'POST', input);

    const period = carePeriodValue(data?.period);

    this.careUpsert(this.care.periods, period);

    return period;

  }

  async deletePeriod(id: string): Promise<void> {

    await this.careRequest('/care/periods/' + id, 'DELETE');

    this.care.periods = this.care.periods.filter(entry => entry.id !== id);

  }

  async loadLogs(from?: string, to?: string): Promise<CycleLog[]> {

    const query = new URLSearchParams();

    if (from) query.set('from', from);

    if (to) query.set('to', to);

    const suffix = query.toString();

    const data = await this.careRequest('/care/logs' + (suffix ? '?' + suffix : ''));

    if (!Array.isArray(data?.logs)) throw new ApiError('invalid_response');

    this.care.logs = data.logs.map(careLogValue);

    return this.care.logs;

  }

  async saveLog(input: { id?: string; log_date: string; pain_level?: number; mood?: string; symptoms?: string[] | null; note?: string }): Promise<CycleLog> {

    const data = await this.careRequest(input.id ? '/care/logs/' + input.id : '/care/logs', input.id ? 'PUT' : 'POST', input);

    const log = careLogValue(data?.log);

    this.careUpsert(this.care.logs, log);

    return log;

  }

  async deleteLog(id: string): Promise<void> {

    await this.careRequest('/care/logs/' + id, 'DELETE');

    this.care.logs = this.care.logs.filter(entry => entry.id !== id);

  }

  async loadCareStatistics(): Promise<CareStatistics> {

    const data = await this.careRequest('/care/statistics');

    const value = data?.statistics;

    if (!isCareObject(value) || !Array.isArray(value.cycles) || typeof value.average_cycle_days !== 'number' || typeof value.average_period_days !== 'number' || value.confidence !== 'estimate') throw new ApiError('invalid_response');

    this.care.statistics = value as CareStatistics;

    return this.care.statistics;

  }

  async loadCarePreferences(): Promise<CarePreferences> {

    const data = await this.careRequest('/care/preferences');

    this.care.preferences = carePreferencesValue(data?.preferences ?? data);

    return this.care.preferences;

  }

  async saveCarePreferences(input: CarePreferences): Promise<CarePreferences> {

    const data = await this.careRequest('/care/preferences', 'PUT', input);

    this.care.preferences = carePreferencesValue(data?.preferences ?? data);

    return this.care.preferences;

  }

  async loadCareShares(): Promise<CareShare[]> {

    const data = await this.careRequest('/care/shares');

    if (!Array.isArray(data?.shares)) throw new ApiError('invalid_response');

    this.care.shares = data.shares.map(careShareValue);

    return this.care.shares;

  }

  async grantCareShare(scope: CareScope): Promise<CareShare[]> {

    const data = await this.careRequest('/care/shares', 'POST', { scope });

    if (data?.share !== undefined) this.careUpsert(this.care.shares, careShareValue(data.share));

    else await this.loadCareShares();

    return this.care.shares;

  }

  async revokeCareShare(id: string): Promise<void> {

    await this.careRequest('/care/shares/' + id, 'DELETE');

    this.care.shares = this.care.shares.filter(entry => entry.id !== id);

  }

  async loadPartnerCare(): Promise<PartnerCare> {

    try {

      const data = await this.careRequest('/care/partner');

      const partner = data?.partner;

      if (!isCareObject(partner) || typeof partner.display_name !== 'string' || !isCareObject(partner.grants)) throw new ApiError('invalid_response');

      this.care.partner = partner as PartnerCare;

      return this.care.partner;

    } catch (error) {

      this.care.partner = null;

      throw error;

    }

  }

  async loadCareTasks(status?: 'open' | 'done'): Promise<CareTask[]> {

    const data = await this.careRequest('/care/tasks' + (status ? '?status=' + status : ''));

    if (!Array.isArray(data?.tasks)) throw new ApiError('invalid_response');

    this.care.tasks = data.tasks.map(careTaskValue);

    return this.care.tasks;

  }

  async saveCareTask(input: { id?: string; title: string; note?: string; due_at?: string | null; status?: 'open' | 'done' }): Promise<CareTask> {

    const data = await this.careRequest(input.id ? '/care/tasks/' + input.id : '/care/tasks', input.id ? 'PUT' : 'POST', input);

    const task = careTaskValue(data?.task);

    this.careUpsert(this.care.tasks, task);

    return task;

  }

  async deleteCareTask(id: string): Promise<void> {

    await this.careRequest('/care/tasks/' + id, 'DELETE');

    this.care.tasks = this.care.tasks.filter(entry => entry.id !== id);

  }

  async loadCareReminders(): Promise<CareReminder[]> {

    const data = await this.careRequest('/care/reminders');

    if (!Array.isArray(data?.reminders)) throw new ApiError('invalid_response');

    this.care.reminders = data.reminders.map(careReminderValue);

    return this.care.reminders;

  }

  async ackCareReminder(id: string): Promise<void> {

    await this.careRequest('/care/reminders/receipts', 'POST', { id });

    this.care.reminders = this.care.reminders.filter(entry => entry.id !== id);

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

        // The notification is posted silently and the alert is played directly:

        // a vendor ROM can mute the notification channel, while the direct motor

        // path is exactly what the in-app vibration test uses.

        const delivery = await this.device.deliverInteraction({ action, title: this.text('interactionTitle'), body: this.text(action), allowSound: false, allowVibrate: false, expiresAt: Date.now() + Math.max(0, data.remaining_ms - (performance.now() - start)) });

        if (!['requested', 'suppressed', 'unsupported', 'failed'].includes(delivery.result)) throw new ApiError('invalid_response');

        result = delivery.result;

        if (this.valid(epoch)) this.state.notificationError = delivery.notificationError || '';

      } catch (e) { result = 'failed'; if (this.valid(epoch)) this.state.notificationError = errorCode(e); }

      if (fresh && allowed && !documentHidden()) {

        try {

          const direct = await this.device.feedback(action === 'ring');

          if (direct === 'requested') result = 'requested';

        } catch { /* the notification path already reported its own state */ }

      }

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

