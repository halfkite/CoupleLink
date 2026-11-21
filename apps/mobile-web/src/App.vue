<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue';
import { APP_VERSION, ApiError, Client, actions, errorCode, normalizeOrigin, newID, parseLoginLink, type Action } from './core';
import { ChatStorage } from './storage';
import { device, transport, native, isAndroid, type NotificationDiagnostics } from './device';
import { text, type Locale } from './strings';
import notices from './notices.txt?raw';
import { AutoReception } from './reception';
import ChatPanel from './ChatPanel.vue';
import CalendarPanel from './CalendarPanel.vue';
import NotesPanel from './NotesPanel.vue';
import CareCenter from './care/CareCenter.vue';
import HomeCare from './home/HomeCare.vue';
import HomeTime from './home/HomeTime.vue';
import HomeBond from './home/HomeBond.vue';
const locale = ref<Locale>(localStorage.getItem('cl.locale') === 'en' ? 'en' : 'zh');
const t = (key: string) => text(key, locale.value);
const remarkDraft = ref('');
const browserOrigin = typeof location !== 'undefined' && /^https?:$/.test(location.protocol) ? location.origin : '';
const client = new Client(transport, new ChatStorage(), device, t); client.state = reactive(client.state); client.memories = reactive(client.memories); client.care = reactive(client.care); const s = client.state;
const tab = ref('home'), carePage = ref<'' | 'care'>(''), theme = ref(localStorage.getItem('cl.theme') || 'sakura');
const careRef = ref<{ back: () => boolean } | null>(null);
const vvHeight = ref(0);
const update = ref<{ version: string; url: string; size?: number; sha256?: string } | null>(null), updateState = ref<'' | 'none' | 'busy'>(''), updatePrompt = ref<{ version: string; url: string; size?: number; sha256?: string } | null>(null), dismissedVersion = ref('');
const profileOpen = ref(false), nicknameDraft = ref(''), avatarPicker = ref<HTMLInputElement>(), startDraft = ref(''), profileSaved = ref(false), settingsPage = ref<string>('');
const settingsTitle = computed(() => { const map: Record<string, string> = { account: t('sectionAccount'), couple: t('sectionCouple'), notify: t('sectionNotify'), look: t('sectionLook'), server: t('server'), about: t('sectionAbout') }; return map[settingsPage.value] || ''; });
watch(() => s.couple?.start_at, value => { if (value) startDraft.value = value; }, { immediate: true });
watch(() => s.notice, value => { if (!value) return; setTimeout(() => { if (s.notice === value) s.notice = ''; }, 5000); });
watch(profileOpen, open => {
  if (!open) return;
  nicknameDraft.value = s.user?.display_name || '';
  remarkDraft.value = s.couple?.my_remark || '';
});
function syncViewport() { vvHeight.value = window.visualViewport?.height ?? window.innerHeight; }
const address = ref(localStorage.getItem('cl.origin') || browserOrigin || 'http://192.168.137.1:18080');
let saved: string[] = []; try { const a = JSON.parse(localStorage.getItem('cl.addresses') || '[]'); if (Array.isArray(a)) saved = a.filter(x => typeof x === 'string'); } catch { /* Invalid optional address preferences are reset, never credentials. */ }
const today = new Date();
const localDate = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`;
let loginSaved: string[] = [];
try { const history = JSON.parse(localStorage.getItem('cl.login.history') || '[]'); if (Array.isArray(history)) loginSaved = history.filter((item: unknown) => typeof item === 'string').slice(0, 5); } catch { /* ignore broken history */ }
const loginHistory = ref<string[]>(loginSaved);
const addresses = ref(saved), invite = ref(''), startDate = ref(localDate), loginLink = ref(localStorage.getItem('cl.login') || loginHistory.value[0] || '');
function recordLogin(value: string) {
  const text = value.trim(); if (!text) return;
  loginHistory.value = [text, ...loginHistory.value.filter(item => item !== text)].slice(0, 5);
  localStorage.setItem('cl.login.history', JSON.stringify(loginHistory.value));
}
function clearLoginHistory() { loginHistory.value = []; localStorage.removeItem('cl.login.history'); }
function shortLogin(value: string) { return value.length > 40 ? value.slice(0, 18) + '…' + value.slice(-14) : value; }
const feedback = ref(''), actionError = ref(''), pending = ref(false), icons: Record<string, string[]> = {
  home: ['M12 20.3S4.6 15.9 4.6 10.9A4.2 4.2 0 0 1 12 8.3a4.2 4.2 0 0 1 7.4 2.6c0 5-7.4 9.4-7.4 9.4z'],
  map: ['M12 20.3s6.4-5.8 6.4-10.3a6.4 6.4 0 1 0-12.8 0c0 4.5 6.4 10.3 6.4 10.3z', 'M12 9.3a1.8 1.8 0 1 0 0 3.6 1.8 1.8 0 0 0 0-3.6z'],
  chat: ['M4.6 6.6a3 3 0 0 1 3-3h8.8a3 3 0 0 1 3 3v5.9a3 3 0 0 1-3 3H9.6l-5 4z'],
  time: ['M12 20.4a8.4 8.4 0 1 0 0-16.8 8.4 8.4 0 0 0 0 16.8z', 'M12 7.8V12l2.9 1.9'],
  me: ['M12 15.1a3.1 3.1 0 1 0 0-6.2 3.1 3.1 0 0 0 0 6.2z', 'M19 12c0 .5 0 1-.1 1.4l1.7 1.3-1.8 3.1-2-.8a7.6 7.6 0 0 1-2.2 1.3l-.3 2.1h-3.6l-.3-2.1a7.6 7.6 0 0 1-2.2-1.3l-2 .8-1.8-3.1 1.7-1.3a8.2 8.2 0 0 1 0-2.8L4 9.3l1.8-3.1 2 .8a7.6 7.6 0 0 1 2.2-1.3l.3-2.1h3.6l.3 2.1a7.6 7.6 0 0 1 2.2 1.3l2-.8 1.8 3.1-1.7 1.3c.1.4.1.9.1 1.4z']
};
const diagnostics = ref<NotificationDiagnostics | null>(null);
const reception = new AutoReception({ context: () => ({ scope: s.ready && s.user && s.couple ? JSON.stringify([s.origin, s.user.id, s.couple.id]) : '', foreground: !document.hidden }), status: () => native.status(), permission: () => native.permission(), start: () => client.background(true), stop: () => client.background(false), wasPrompted: () => localStorage.getItem('cl.notifications.prompted') === 'yes', markPrompted: () => localStorage.setItem('cl.notifications.prompted', 'yes'), update: value => { s.background = value; } });
const actionIcons: Record<Action, string[]> = {
  poke: ['M9 11.2V6.6a1.5 1.5 0 0 1 3 0v4.2', 'M12 10.8V5.6a1.5 1.5 0 0 1 3 0v5.2', 'M15 10.9V7.4a1.5 1.5 0 0 1 3 0v6.2a5.8 5.8 0 0 1-5.8 5.8h-1.1a4.8 4.8 0 0 1-3.7-1.8l-2.8-3.5a1.55 1.55 0 0 1 2.3-2.1L9 14.2'],
  miss_you: ['M12 20.3S4.6 15.9 4.6 10.9A4.2 4.2 0 0 1 12 8.3a4.2 4.2 0 0 1 7.4 2.6c0 5-7.4 9.4-7.4 9.4z'],
  hug: ['M8.9 9.4a2.9 2.9 0 1 0 0 5.4c1.7 0 2.5-1.2 3.1-2.6.6 1.4 1.4 2.6 3.1 2.6a2.9 2.9 0 1 0 0-5.4c-1.7 0-2.5 1.2-3.1 2.6-.6-1.4-1.4-2.6-3.1-2.6z'],
  vibrate: ['M9.2 4.6h5.6a2 2 0 0 1 2 2v10.8a2 2 0 0 1-2 2H9.2a2 2 0 0 1-2-2V6.6a2 2 0 0 1 2-2z', 'M4.6 9.4v5.2M19.4 9.4v5.2'],
  ring: ['M18.1 16.2H5.9l1.7-2.3V10a4.4 4.4 0 0 1 8.8 0v3.9l1.7 2.3z', 'M10.4 19.1a1.7 1.7 0 0 0 3.2 0'],
};
let timer: ReturnType<typeof setInterval> | undefined;
const days = computed(() => s.couple?.days_together ?? '—');
const partnerName = computed(() => s.couple?.my_remark || s.couple?.partner.display_name || '');
const partnerGender = computed(() => s.couple?.partner.gender === 'female' ? 'female' : s.couple?.partner.gender === 'male' ? 'male' : '');
const adminURL = computed(() => { try { return new URL('/admin', s.origin || browserOrigin || 'http://127.0.0.1:30710').href; } catch { return '/admin'; } });
async function saveRemark() { await client.setRemark(remarkDraft.value); feedback.value = t('saved'); }
watch(() => s.couple?.my_remark, value => { remarkDraft.value = value || ''; }, { immediate: true });
async function run(action: () => Promise<unknown>) { if (pending.value) return; pending.value = true; feedback.value = ''; actionError.value = ''; try { await action(); } catch (e) { actionError.value = errorCode(e); } finally { pending.value = false; } }
async function selectServer(value: string) { await client.changeServer(value); address.value = s.origin; localStorage.setItem('cl.origin', s.origin); if (!addresses.value.includes(s.origin)) addresses.value.push(s.origin); }
function resolveLogin(value: string): { origin: string; key: string } {
  const text = value.trim();
  if (/^key[a-z0-9]{16,32}$/.test(text)) {
    if (!browserOrigin) throw new ApiError('invalid_login_link');
    return { origin: normalizeOrigin(browserOrigin), key: text };
  }
  return parseLoginLink(text);
}
async function login() {
  const typed = loginLink.value || address.value;
  recordLogin(typed);
  const { origin, key } = resolveLogin(typed);
  const deviceKey = `cl.device.${origin}`;
  let id = localStorage.getItem(deviceKey); if (!id) { id = newID(); localStorage.setItem(deviceKey, id); }
  await client.authenticateKey(origin, key, id, '');
  address.value = s.origin; localStorage.setItem('cl.login', loginLink.value); await client.sync();
}
async function refreshDiagnostics() { if (!isAndroid) { feedback.value = t('android_only'); return; } diagnostics.value = await native.diagnostics(); s.background = diagnostics.value.running; }
async function permission() { if (!isAndroid) { feedback.value = t('android_only'); return; } const granted = await reception.ensure(true); feedback.value = t(granted ? 'granted' : 'denied'); if (granted) s.notificationError = ''; await refreshDiagnostics(); }
async function autoReception() { if (isAndroid) { try { await reception.ensure(); } catch (e) { s.notificationError = errorCode(e); } } }
async function notifyTest() { if (!isAndroid) { feedback.value = t('android_only'); return; } await refreshDiagnostics(); await native.notify({ kind: 'test', title: 'CoupleLink', body: t('newMessage') }); feedback.value = t('requested'); }
async function feedbackTest(sound: boolean) {
  feedback.value = t(await device.feedback(sound));
}
async function confirmUnbind() { if (window.confirm(t('confirmUnbind'))) await client.unbind(); }
watch(addresses, value => localStorage.setItem('cl.addresses', JSON.stringify(value)), { deep: true });
watch(theme, value => localStorage.setItem('cl.theme', value)); watch(locale, value => localStorage.setItem('cl.locale', value));
watch(tab, value => { if (value !== 'chat') void run(() => client.setChatVisible(false)); });
watch(() => s.ready ? JSON.stringify([s.origin, s.user?.id, s.couple?.id]) : '', scope => { if (scope) void autoReception(); });
watch(() => s.cursor, () => { if (isAndroid && s.couple && s.user) void native.baseline({ origin: s.origin, user: s.user.id, couple: s.couple.id, cursor: s.cursor }).catch(e => { s.error = errorCode(e); }); });
async function changeAvatar(event: Event) {
  const element = event.target as HTMLInputElement, file = element.files?.[0];
  element.value = '';
  if (!file) return;
  await run(async () => {
    if (file.size > 5_500_000) throw new ApiError('image_too_large');
    const base64 = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => { const value = String(reader.result || ''); resolve(value.slice(value.indexOf(',') + 1)); };
      reader.onerror = () => reject(new ApiError('image_failed'));
      reader.readAsDataURL(file);
    });
    await client.updateAvatar(await client.uploadImage(base64));
  });
}
async function saveStart() {
  if (!s.couple) { s.notice = t('not_bound'); return; }
  try {
    await client.updateStartDate(startDraft.value, s.couple.version);
    s.notice = t('saved');
  } catch (e) { s.notice = t(errorCode(e)); }
}
async function saveProfile() {
  // Saved outside run() so a stuck busy state can never swallow this action.
  if (pending.value) return;
  pending.value = true;
  try {
    await client.updateProfile(nicknameDraft.value);
    await client.setRemark(remarkDraft.value);
    profileOpen.value = false;
    profileSaved.value = true;
    setTimeout(() => { profileSaved.value = false; }, 2500);
  } catch (error) {
    console.error('saveProfile failed', error);
    s.notice = t(errorCode(error)) + ' [' + errorCode(error) + ']';
  } finally {
    pending.value = false;
  }
}
async function checkUpdate() {
  updateState.value = 'busy'; update.value = null;
  try {
    update.value = await client.checkUpdate();
    updateState.value = update.value ? '' : 'none';
    if (update.value) updatePrompt.value = update.value;
  } catch (e) { s.notice = t(errorCode(e)); updateState.value = ''; }
}
async function autoCheckUpdate() {
  try {
    const found = await client.checkUpdate();
    if (found && found.version !== dismissedVersion.value) updatePrompt.value = found;
  } catch { /* an offline start simply skips the check */ }
}
function dismissUpdate() { if (updatePrompt.value) dismissedVersion.value = updatePrompt.value.version; updatePrompt.value = null; }
async function openUpdate() {
  const target = updatePrompt.value || update.value;
  if (!target) return;
  try { updatePrompt.value = null; await device.downloadAndInstall?.(target.url, target.sha256); s.notice = t('downloadingUpdate'); } catch (e) { s.notice = t(errorCode(e)); }
}
// A paired couple on an old server is told why the center cannot open; an unpaired
// member may still enter and the center asks them to pair first.
function openCare() {
  if (s.couple && !client.care.supported) { s.notice = t('careUnsupported'); return; }
  carePage.value = 'care';
  if (client.care.supported) void client.loadCareDashboard().catch(() => { /* the center renders its own error state */ });
}
// Android's hardware back is forwarded here by MainActivity. Unwind one screen at a
// time so a back press inside settings or care never closes the whole app.
function backEvent() {
  if (carePage.value === 'care') {
    if (careRef.value?.back()) return;
    carePage.value = '';
    return;
  }
  if (updatePrompt.value) { dismissUpdate(); return; }
  if (profileOpen.value) { profileOpen.value = false; return; }
  if (settingsPage.value) { settingsPage.value = ''; return; }
  if (tab.value !== 'home') { tab.value = 'home'; return; }
  void device.exitApp?.();
}
function notificationTap() { void run(async () => { carePage.value = ''; tab.value = isAndroid && (await native.launch()).kind === 'anniversary' ? 'time' : 'chat'; }); }
async function visibility() { if (!document.hidden) { await client.sync(); await autoReception(); } }
onMounted(() => { void run(async () => { if (isAndroid) { s.background = (await native.status()).running;  } await client.restore(); if (isAndroid) { const kind = (await native.launch()).kind; if(kind === 'messages' || kind === 'anniversary') tab.value = kind === 'anniversary' ? 'time' : 'chat'; } if (s.origin) address.value = s.origin; await client.sync(); void autoCheckUpdate(); }); timer = setInterval(() => { if (!document.hidden) void client.sync(); }, 2000); document.addEventListener('visibilitychange', visibility); window.addEventListener('couplelink-notification', notificationTap); window.addEventListener('couplelink-back', backEvent); syncViewport(); window.visualViewport?.addEventListener('resize', syncViewport); window.addEventListener('resize', syncViewport); });
onUnmounted(() => { if (timer) clearInterval(timer); document.removeEventListener('visibilitychange', visibility); window.removeEventListener('couplelink-notification', notificationTap); window.removeEventListener('couplelink-back', backEvent); window.visualViewport?.removeEventListener('resize', syncViewport); window.removeEventListener('resize', syncViewport); });
</script>

<template>
  <div class="app" :data-theme="theme" :class="{ 'chat-mode': tab === 'chat' }" :style="vvHeight ? { '--app-h': vvHeight + 'px' } : undefined">

    <main>
      <div class="feedback-stack">
      <div v-if="actionError" class="banner error" role="alert">{{ t(actionError) }}<button @click="actionError = ''" :aria-label="t('close')">×</button></div>
      <div v-if="s.error && !s.offline" class="banner error" role="alert">{{ t(s.error) }}</div>
      <div v-if="s.notificationError" class="banner error" role="alert">{{ t(s.notificationError) }}</div>
      <div v-if="feedback" class="banner" role="status">{{ feedback }}<button @click="feedback = ''" :aria-label="t('close')">×</button></div>
      <div v-if="s.offline" class="banner" role="status">离线模式 · 显示本地缓存的聊天记录</div>
<div v-if="s.notice" class="banner prominent" role="status">{{ t(s.notice) }} <button @click="s.notice = ''" :aria-label="t('close')">×</button></div>
      </div>
      <template v-if="carePage !== 'care'">
      <section v-if="tab === 'home'">
        <HomeBond :client="client" :t="t" @open="target => tab = target" />
        <article class="card action-card"><div class="action-grid"><button v-for="action in actions" :key="action" type="button" class="action-tile" :disabled="pending || !s.couple" :aria-label="t(action)" @click="run(() => client.interact(action))"><span class="action-bubble"><svg viewBox="0 0 24 24" aria-hidden="true"><path v-for="d in actionIcons[action]" :key="d" :d="d" /></svg></span><span class="action-label">{{ t(action) }}</span></button></div></article>
        
        <HomeCare :client="client" :t="t" @open="openCare" />
        <HomeTime :client="client" :t="t" @open="tab = 'time'" />
      </section>
      <section v-if="tab === 'chat'"><ChatPanel :client="client" :t="t" /></section>
      <section v-if="tab === 'time'"><div class="detail-head"><button type="button" class="back" :aria-label="t('backHome')" @click="tab = 'home'">‹</button><h1>{{ t('time') }}</h1></div><NotesPanel :client="client" :t="t" /><details class="card anniversaries"><summary>{{ t('anniversaries') }}</summary><CalendarPanel :client="client" :t="t" /></details></section>
      <section v-if="tab === 'map'"><div class="heading"><h1>{{ t('map') }}</h1><p>{{ t('featurePending') }}</p></div><article class="card empty"><span class="empty-icon">⌖</span><p>{{ t('mapHelp') }}</p></article></section>
      <section v-if="tab === 'me'">
        <template v-if="!settingsPage">
        <div class="heading"><h1>{{ t('me') }}</h1><p>CoupleLink · {{ APP_VERSION }}</p></div>
        <p v-if="actionError" class="banner error" role="alert">{{ t(actionError) }}</p>
        <button class="set-row" type="button" @click="settingsPage = 'account'"><span class="set-ic"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 12.4a3.6 3.6 0 1 0 0-7.2 3.6 3.6 0 0 0 0 7.2zM5 20a7 7 0 0 1 14 0" /></svg></span><span class="set-text"><b>{{ t('sectionAccount') }}</b><small>{{ s.user ? s.user.display_name : t('sectionAccount') }}</small></span><i class="chev">›</i></button>
        <button class="set-row" type="button" @click="settingsPage = 'couple'"><span class="set-ic"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 4.5v3M17 4.5v3M4.5 9.5h15M6 6.5h12a1.5 1.5 0 0 1 1.5 1.5v10.5A1.5 1.5 0 0 1 18 20H6a1.5 1.5 0 0 1-1.5-1.5V8A1.5 1.5 0 0 1 6 6.5z" /></svg></span><span class="set-text"><b>{{ t('sectionCouple') }}</b><small>{{ s.couple ? s.couple.start_at : t('date') }}</small></span><i class="chev">›</i></button>
        <button class="set-row" type="button" @click="settingsPage = 'notify'"><span class="set-ic"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M18.1 16.2H5.9l1.7-2.3V10a4.4 4.4 0 0 1 8.8 0v3.9l1.7 2.3zM10.4 19.1a1.7 1.7 0 0 0 3.2 0" /></svg></span><span class="set-text"><b>{{ t('sectionNotify') }}</b><small>{{ t('notifyHint') }}</small></span><i class="chev">›</i></button>
        <button class="set-row" type="button" @click="settingsPage = 'look'"><span class="set-ic"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 4.5a7.5 7.5 0 0 1 7.5 7.5c0 4.1-3.4 7.5-7.5 7.5S4.5 16.1 4.5 12 7.9 4.5 12 4.5zM12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6z" /></svg></span><span class="set-text"><b>{{ t('sectionLook') }}</b><small>{{ t('lookHint') }}</small></span><i class="chev">›</i></button>
        <button class="set-row" type="button" @click="settingsPage = 'server'"><span class="set-ic"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4.5 7.5h15M4.5 12h15M4.5 16.5h15M7.5 7.5v.01M7.5 12v.01M7.5 16.5v.01" /></svg></span><span class="set-text"><b>{{ t('server') }}</b><small>{{ s.origin }}</small></span><i class="chev">›</i></button>
        <button class="set-row" type="button" @click="settingsPage = 'about'"><span class="set-ic"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 20.4a8.4 8.4 0 1 0 0-16.8 8.4 8.4 0 0 0 0 16.8zM12 10.6v5.4M12 7.8v.6" /></svg></span><span class="set-text"><b>{{ t('sectionAbout') }}</b><small>{{ t('currentVersion') }} {{ APP_VERSION }}</small></span><i class="chev">›</i></button>
                </template>
        <template v-else>
          <div class="detail-head"><button type="button" class="back" :aria-label="t('back')" @click="settingsPage = ''">‹</button><h1>{{ settingsTitle }}</h1></div>
        <article v-if="settingsPage === 'account'" class="card"><h2>{{ t('account') }}</h2><p v-if="!isAndroid" class="subtle">{{ t('browserHelp') }}</p><p class="subtle">{{ t('migrationHelp') }}</p>
          <template v-if="!s.user"><form @submit.prevent="run(login)"><label>{{ t('loginLink') }}<input v-model="loginLink" inputmode="url" autocomplete="off" spellcheck="false" required placeholder="http://127.0.0.1:30710-keyxxxxxxxxxxxxxxxxxxxx"></label><p class="subtle">{{ t('loginLinkHelp') }}</p><p v-if="browserOrigin" class="subtle">{{ t('loginKeyOnly') }}</p><div v-if="loginHistory.length" class="login-history"><p class="subtle">{{ t('loginHistory') }}</p><div class="history-row"><button v-for="item in loginHistory" :key="item" type="button" class="history-chip" @click="loginLink = item">{{ shortLogin(item) }}</button></div><button type="button" class="text-button" @click="clearLoginHistory">{{ t('clearHistory') }}</button></div><button class="primary wide" :disabled="pending">{{ t(pending ? 'busy' : 'login') }}</button></form></template>
          <template v-else><div class="profile"><button class="avatar" type="button" :aria-label="t('editProfile')" @click="profileOpen = !profileOpen"><img v-if="s.couple?.my_avatar && client.imageURL(s.couple.my_avatar)" class="avatar-img" :src="client.imageURL(s.couple.my_avatar)" alt="" /><template v-else>{{ s.user.display_name.slice(0, 1) }}</template></button><div><h3>{{ s.user.display_name }}</h3><p>@{{ s.user.username }}</p></div></div><template v-if="!s.couple"><button class="outline wide" :disabled="pending" @click="run(() => client.invitation())">{{ t('createInvite') }}</button><output v-if="s.invitation" class="invite">{{ s.invitation }}</output><form @submit.prevent="run(() => client.join(invite, startDate))"><label>{{ t('invitation') }}<input v-model="invite" required></label><label>{{ t('date') }}<input type="date" v-model="startDate" required></label><button class="primary wide" :disabled="pending">{{ t('join') }}</button></form></template><template v-else><p>♡ {{ partnerName }} </p><p v-if="s.couple.partner_remark" class="subtle">{{ t('partnerRemarkForMe') }}: {{ s.couple.partner_remark }}</p><div v-if="profileOpen" class="profile-form"><div class="avatar-edit"><button type="button" class="outline pill" @click="avatarPicker?.click()">{{ t('changeAvatar') }}</button><input ref="avatarPicker" class="file-input" type="file" accept="image/*" @change="changeAvatar" /></div><label>{{ t('nickname') }}<input v-model="nicknameDraft" maxlength="24" required></label><label>{{ t('remarkForPartner') }}<input v-model="remarkDraft" maxlength="40" :placeholder="s.couple.partner.display_name"></label><button type="button" class="primary wide" :disabled="pending" @click="saveProfile">{{ profileSaved ? '✓ ' + t('saved') : t('saveChanges') }}</button></div><button v-else class="outline wide" @click="profileOpen = true">{{ t('editProfile') }}</button><p class="subtle">{{ profileSaved ? t('profileSavedHint') : t('remarkHelp') }}</p><button class="outline" :disabled="pending" @click="run(confirmUnbind)">{{ t('unbind') }}</button></template><button class="text-button" :disabled="pending" @click="run(() => client.logout())">{{ t('logout') }}</button></template>
        </article>
        <article v-if="s.couple && settingsPage === 'couple'" class="card"><h2>{{ t('date') }}</h2><p class="subtle">{{ t('startDateHelp') }}</p><input type="date" v-model="startDraft" @change="saveStart" /><button type="button" class="primary wide" :disabled="pending" @click="saveStart">{{ t('saveChanges') }}</button></article>
        <article v-if="settingsPage === 'notify'" class="card"><h2>{{ t('notifications') }}</h2><p class="background-status" role="status">{{ t(s.background ? 'backgroundRunning' : 'backgroundStopped') }}</p><h3>{{ t('background') }}</h3><p class="subtle">{{ t('backgroundHelp') }}</p><button v-if="!s.background" class="primary wide" :disabled="pending" @click="run(permission)">{{ t('enableReminders') }}</button><div class="button-grid"><button class="outline" @click="run(notifyTest)">{{ t('notificationTest') }}</button><button class="outline" @click="run(() => feedbackTest(false))">{{ t('vibrationTest') }}</button><button class="outline" @click="run(() => feedbackTest(true))">{{ t('soundTest') }}</button></div><button class="text-button" @click="run(async () => { if(isAndroid) await native.settings(); else feedback = t('android_only') })">{{ t('systemSettings') }} ↗</button><button class="outline wide" @click="run(refreshDiagnostics)">{{ t('diagnostics') }}</button>
<details v-if="diagnostics" class="diagnostics"><summary>{{ t('technicalDiagnostics') }}</summary>
  <p class="subtle">{{ diagnostics.manufacturer }} {{ diagnostics.model }} · Android API {{ diagnostics.sdk }}</p>
  <dl><dt>{{ t('notificationPermission') }}</dt><dd>{{ t(diagnostics.notificationsEnabled ? 'enabled' : 'blocked') }}</dd><dt>{{ t('vibrationHardware') }}</dt><dd>{{ t(diagnostics.hasVibrator ? 'available' : 'unsupported') }}</dd><dt>{{ t('vibrationOperation') }}</dt><dd>{{ t(diagnostics.vibrationOperation === 'allowed' ? 'enabled' : diagnostics.vibrationOperation === 'blocked' ? 'blocked' : 'unknown') }}</dd><dt>{{ t('hapticFeedback') }}</dt><dd>{{ t(diagnostics.hapticFeedbackEnabled ? 'enabled' : 'disabled') }}</dd><dt>{{ t('notificationVibration') }}</dt><dd>{{ t(diagnostics.notificationVibration ? 'enabled' : 'disabled') }}</dd><dt>{{ t('vibrationMethod') }}</dt><dd>{{ t(diagnostics.feedbackMethod === 'touch' ? 'hapticFeedback' : diagnostics.feedbackMethod === 'notification' ? 'notificationVibration' : 'methodDisabled') }}</dd><dt>{{ t('soundMode') }}</dt><dd>{{ t(diagnostics.ringerMode === 0 ? 'silent' : diagnostics.ringerMode === 1 ? 'vibrationMode' : 'normalMode') }}</dd><dt>{{ t('dnd') }}</dt><dd>{{ t(diagnostics.interruptionFilter === 1 ? 'disabled' : diagnostics.interruptionFilter === 0 ? 'unknown' : 'enabled') }}</dd><dt>{{ t('background') }}</dt><dd>{{ t(diagnostics.running ? 'running' : 'stopped') }}</dd></dl>
  <p v-if="diagnostics.feedbackMethod === 'disabled'" class="subtle">{{ t('vibration_disabled') }}</p>
  <p v-else class="subtle">{{ t('hapticHelp') }}</p>
  <p class="subtle">{{ t('headsUpHelp') }}</p><details><summary>{{ t('diagnosticReport') }}</summary><textarea readonly rows="10" :value="JSON.stringify(diagnostics, null, 2)" :aria-label="t('diagnosticReport')"></textarea></details></details>
</article>
        <article v-if="settingsPage === 'look'" class="card"><h2>{{ t('appearance') }}</h2><div class="themes"><button v-for="name in ['sakura', 'lavender', 'midnight']" :key="name" :class="{ active: theme === name }" @click="theme = name">{{ t(name) }}</button></div><label>{{ t('language') }}<select v-model="locale" :aria-label="t('language')"><option value="zh">简体中文</option><option value="en">English</option></select></label><details><summary>{{ t('licenses') }}</summary><pre class="licenses">{{ notices }}</pre></details></article>
        <article v-if="settingsPage === 'about'" class="card"><h2>{{ t('updates') }}</h2><p class="subtle">{{ t('currentVersion') }} {{ APP_VERSION }}</p><button class="outline wide" type="button" :disabled="updateState === 'busy'" @click="run(checkUpdate)">{{ t('checkUpdate') }}</button><p v-if="updateState === 'none'" class="subtle">{{ t('upToDate') }}</p><div v-if="update" class="update-box"><p>{{ t('updateAvailable') }} <strong>{{ update.version }}</strong></p><button class="primary wide" type="button" @click="openUpdate">{{ t('download') }}</button></div></article>
        <article v-if="browserOrigin && settingsPage === 'about'" class="card"><h2>{{ t('openAdmin') }}</h2><p class="subtle">{{ adminURL }}</p><a class="text-button" :href="adminURL" target="_blank" rel="noopener">{{ t('openAdmin') }} ↗</a></article>
        <article v-if="settingsPage === 'server'" class="card"><h2>{{ t('addresses') }}</h2><label>{{ t('server') }}<input v-model="address" inputmode="url"></label><p v-if="address.startsWith('http:')" class="warning">{{ t('httpWarning') }}</p><div class="button-grid"><button class="outline" @click="run(async () => { feedback = t('testOk') + ' · ' + await client.test(address) })">{{ t('test') }}</button><button class="outline" @click="run(async () => { const value = normalizeOrigin(address); if(!addresses.includes(value)) addresses.push(value) })">{{ t('addAddress') }}</button></div><p class="subtle">{{ t('manualAddresses') }}</p><div v-for="value in addresses" :key="value" class="address-row"><button :disabled="pending || value === s.origin" @click="run(() => selectServer(value))">{{ value }}<small v-if="value === s.origin">{{ t('selected') }}</small></button><button @click="addresses = addresses.filter(x => x !== value)" :aria-label="t('remove')">×</button></div><button class="primary wide" :disabled="pending" @click="run(() => selectServer(address))">{{ t('save') }}</button></article>
        </template>
      </section>
      </template>
      <section v-if="carePage === 'care'"><CareCenter ref="careRef" :client="client" :t="t" @close="carePage = ''" /></section>
    </main>
    <nav v-if="carePage !== 'care'" :aria-label="t('space')"><button v-for="name in ['home', 'map', 'chat', 'me']" :key="name" :class="{ active: tab === name }" @click="tab = name"><span class="nav-icon"><svg class="nav-svg" viewBox="0 0 24 24" aria-hidden="true"><path v-for="d in icons[name]" :key="d" :d="d" /></svg><i v-if="name === 'chat' && s.unread" class="unread" :aria-label="t('newMessage')"></i></span>{{ t(name) }}</button></nav>
    <div v-if="updatePrompt" class="modal-backdrop"><section class="interaction-dialog update-dialog" role="dialog" aria-modal="true" :aria-label="t('updateAvailable')"><p>{{ t('updateAvailable') }}</p><h1>{{ updatePrompt.version }}</h1><p class="subtle">{{ t('updateExplain') }}</p><button class="primary wide" @click="openUpdate">{{ t('updateNow') }}</button><button class="text-button" @click="dismissUpdate">{{ t('later') }}</button></section></div>
    <div v-if="s.incoming" class="modal-backdrop"><section class="interaction-dialog" role="dialog" aria-modal="true" :aria-label="t('interactionTitle')"><p>{{ t('interactionTitle') }}</p><div class="heart"><svg viewBox="0 0 24 24" aria-hidden="true"><path v-for="d in actionIcons[s.incoming]" :key="d" :d="d" /></svg></div><h1>{{ t(s.incoming) }}</h1><button class="primary wide" autofocus @click="s.incoming = ''">{{ t('close') }}</button></section></div>
  </div>
</template>
