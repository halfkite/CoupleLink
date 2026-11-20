import { Capacitor, registerPlugin } from '@capacitor/core';
import type { Device, Session, Transport } from './core';
import { ApiError, normalizeOrigin } from './core';
interface NativeDevice {
  sessionRead(): Promise<{ value: string | null }>; sessionWrite(options: { value: string | null }): Promise<void>;
  request(options: { origin: string; path: string; method: string; body?: unknown; token?: string }): Promise<{ status: number; data: any }>;
  feedback(options: { sound: boolean }): Promise<{ result: string }>;
  notify(options: { kind: string; title: string; body: string }): Promise<void>;
  permission(): Promise<{ granted: boolean }>; settings(options?: { channel: string }): Promise<void>;
  interaction(options: Parameters<NonNullable<Device['deliverInteraction']>>[0]): Promise<{ result: string; notificationError?: string }>;
  diagnostics(): Promise<NotificationDiagnostics>;
  background(options: Parameters<Device['background']>[0]): Promise<void>;
  baseline(options: { origin: string; user: string; couple: string; cursor: number }): Promise<void>;
  status(): Promise<{ running: boolean; notificationsEnabled: boolean }>;
  launch(): Promise<{ kind: string | null }>;
  exitApp(): Promise<void>;
  openUrl(options: { url: string }): Promise<void>;
  downloadAndInstall(options: { url: string; sha256?: string }): Promise<{ started: boolean }>;
  fetchImage(options: { origin: string; id: string; token: string }): Promise<{ contentType: string; data: string }>;
}
export type NotificationDiagnostics = { sdk: number; manufacturer: string; model: string; notificationsEnabled: boolean; hasVibrator: boolean; ringerMode: number; interruptionFilter: number; running: boolean; vibrationOperation: string; hapticFeedbackEnabled: boolean; notificationVibration: boolean; feedbackMethod: string; channels: { id: string; exists: boolean; importance?: number; sound?: boolean; vibration?: boolean; pattern?: boolean }[] };
export const native = registerPlugin<NativeDevice>('CoupleDevice');
export const isAndroid = Capacitor.getPlatform() === 'android';
let webSession: Session | null = null;
export const device: Device = {
  async sessionRead() { if (!isAndroid) return webSession; const { value } = await native.sessionRead(); return value ? JSON.parse(value) : null; },
  async sessionWrite(value) { if (isAndroid) await native.sessionWrite({ value: value ? JSON.stringify(value) : null }); else webSession = value; },
  async feedback(sound) { return isAndroid ? (await native.feedback({ sound })).result : 'unsupported'; },
  async openUrl(url) { if (isAndroid) await native.openUrl({ url }); else window.open(url, '_blank', 'noopener'); },
  async fetchImage(origin, id, token) { if (!isAndroid) return null; return native.fetchImage({ origin, id, token }); },
  async downloadAndInstall(url, sha256) { if (isAndroid) { await native.downloadAndInstall({ url, sha256 }); return; } window.open(url, '_blank', 'noopener'); },
  async notify(kind, title, body) { if (isAndroid) await native.notify({ kind, title, body }); else if(kind === 'anniversary') throw new ApiError('android_only'); },
  async deliverInteraction(options) { return isAndroid ? native.interaction(options) : { result: options.expiresAt > Date.now() && (options.action === 'ring' ? options.allowSound : options.allowVibrate) ? 'unsupported' : 'suppressed' }; },
  async background(options) { if (isAndroid) await native.background(options); else if (options.enabled) throw new ApiError('android_only'); },
  // A hardware back press at the root of the app leaves it; every inner screen is
  // unwound by the web layer first.
  async exitApp() { if (isAndroid) await native.exitApp(); },
};
export const transport: Transport = async (origin, path, method, body, token) => {
  normalizeOrigin(origin);
  if (!path.startsWith('/') || path.startsWith('//') || path.includes('://')) throw new ApiError('invalid_input');
  if (isAndroid) return native.request({ origin, path, method, body, token });
  const controller = new AbortController(), timer = setTimeout(() => controller.abort(), 10000);
  try {
    const response = await fetch(origin + path, { method, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: body === undefined ? undefined : JSON.stringify(body), redirect: 'error', signal: controller.signal });
    return { status: response.status, data: await response.json() };
  } finally { clearTimeout(timer); }
};
