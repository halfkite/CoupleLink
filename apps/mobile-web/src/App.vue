<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue';
import { Client, actions, errorCode, normalizeOrigin, newID, type Action } from './core';
import { ChatStorage } from './storage';
import { device, transport, native, isAndroid, type NotificationDiagnostics } from './device';
import { text, type Locale } from './strings';
import notices from './notices.txt?raw';
import { AutoReception } from './reception';
import ChatPanel from './ChatPanel.vue';
import CalendarPanel from './CalendarPanel.vue';
const locale = ref<Locale>(localStorage.getItem('cl.locale') === 'en' ? 'en' : 'zh');
const t = (key: string) => text(key, locale.value);
const client = new Client(transport, new ChatStorage(), device, t); client.state = reactive(client.state); client.memories = reactive(client.memories); const s = client.state;
const tab = ref('home'), theme = ref(localStorage.getItem('cl.theme') || 'sakura');
const address = ref(localStorage.getItem('cl.origin') || 'http://192.168.137.1:18080');
let saved: string[] = []; try { const a = JSON.parse(localStorage.getItem('cl.addresses') || '[]'); if (Array.isArray(a)) saved = a.filter(x => typeof x === 'string'); } catch { /* Invalid optional address preferences are reset, never credentials. */ }
const today = new Date();
const localDate = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`;
const addresses = ref(saved), register = ref(false), username = ref(''), password = ref(''), nickname = ref(''), invite = ref(''), startDate = ref(localDate);
const feedback = ref(''), actionError = ref(''), pending = ref(false), icons: Record<string, string> = { home: '♡', map: '⌖', chat: '☏', time: '◷', me: '☺' };
const diagnostics = ref<NotificationDiagnostics | null>(null);
const reception = new AutoReception({ context: () => ({ scope: s.ready && s.user && s.couple ? JSON.stringify([s.origin, s.user.id, s.couple.id]) : '', foreground: !document.hidden }), status: () => native.status(), permission: () => native.permission(), start: () => client.background(true), stop: () => client.background(false), wasPrompted: () => localStorage.getItem('cl.notifications.prompted') === 'yes', markPrompted: () => localStorage.setItem('cl.notifications.prompted', 'yes'), update: value => { s.background = value; } });
const actionIcons: Record<Action, string> = { poke: '☞', miss_you: '♡', hug: '∞', vibrate: '〰', ring: '♧' };
let timer: ReturnType<typeof setInterval> | undefined;
const days = computed(() => s.couple?.days_together ?? '—');
async function run(action: () => Promise<unknown>) { if (pending.value) return; pending.value = true; feedback.value = ''; actionError.value = ''; try { await action(); } catch (e) { actionError.value = errorCode(e); } finally { pending.value = false; } }
async function selectServer(value: string) { await client.changeServer(value); address.value = s.origin; localStorage.setItem('cl.origin', s.origin); if (!addresses.value.includes(s.origin)) addresses.value.push(s.origin); }
async function login() {
  if (s.origin !== address.value.trim().replace(/\/$/, '')) await selectServer(address.value);
  const key = `cl.device.${JSON.stringify([s.origin, username.value.trim().toLowerCase()])}`;
  let id = localStorage.getItem(key); if (!id) { id = newID(); localStorage.setItem(key, id); }
  await client.authenticate(username.value, password.value, nickname.value, register.value, id); password.value = ''; await client.sync();
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
function notificationTap() { void run(async () => { tab.value = isAndroid && (await native.launch()).kind === 'anniversary' ? 'time' : 'chat'; }); }
async function visibility() { if (!document.hidden) { await client.sync(); await autoReception(); } }
onMounted(() => { void run(async () => { if (isAndroid) { s.background = (await native.status()).running;  } await client.restore(); if (isAndroid) { const kind = (await native.launch()).kind; if(kind === 'messages' || kind === 'anniversary') tab.value = kind === 'anniversary' ? 'time' : 'chat'; } if (s.origin) address.value = s.origin; await client.sync(); }); timer = setInterval(() => { if (!document.hidden) void client.sync(); }, 2000); document.addEventListener('visibilitychange', visibility); window.addEventListener('couplelink-notification', notificationTap); });
onUnmounted(() => { if (timer) clearInterval(timer); document.removeEventListener('visibilitychange', visibility); window.removeEventListener('couplelink-notification', notificationTap); });
</script>

<template>
  <div class="app" :data-theme="theme">
    <header><div class="brand">CoupleLink <span>1.0-r11</span></div><button class="avatar small" @click="tab = 'me'" :aria-label="t('me')">♡</button></header>
    <main>
      <div class="feedback-stack">
      <div v-if="actionError" class="banner error" role="alert">{{ t(actionError) }}<button @click="actionError = ''" :aria-label="t('close')">×</button></div>
      <div v-if="s.error" class="banner error" role="alert">{{ t(s.error) }}</div>
      <div v-if="s.notificationError" class="banner error" role="alert">{{ t(s.notificationError) }}</div>
      <div v-if="feedback" class="banner" role="status">{{ feedback }}<button @click="feedback = ''" :aria-label="t('close')">×</button></div>
      <div v-if="s.notice" class="banner prominent" role="status">{{ t(s.notice) }} <button @click="s.notice = ''" :aria-label="t('close')">×</button></div>
      </div>
      <section v-if="tab === 'home'">
        <div class="heading"><p class="eyebrow">JUST THE TWO OF US</p><h1>{{ t('space') }} <span>♡</span></h1><p>{{ t('subtitle') }}</p></div>
        <article class="love-card"><div class="couple-avatars"><div class="avatar">{{ s.user?.display_name?.slice(0, 1) || '♡' }}</div><span>♥</span><div class="avatar partner">{{ s.couple?.partner.display_name?.slice(0, 1) || '♡' }}</div></div><p>{{ t('days') }}</p><div class="days">{{ days }} <small>{{ t('dayUnit') }}</small></div><div class="names">{{ s.user?.display_name || 'CoupleLink' }} <span>&</span> {{ s.couple?.partner.display_name || '♡' }}</div><p class="subtle">{{ s.couple ? s.couple.start_at : t(s.user ? 'bindFirst' : 'connectFirst') }}</p><button v-if="s.couple" class="text-button" @click="tab = 'time'">{{ t('editStartDate') }} →</button></article>
        <article class="card"><h2>{{ t('interactions') }} <span>↗</span></h2><div class="actions"><button v-for="action in actions" :key="action" :disabled="pending || !s.couple" @click="run(() => client.interact(action))"><span>{{ actionIcons[action] }}</span>{{ t(action) }}</button></div></article>
        <article class="card status-card"><span class="status-dot" :class="{ online: s.online }"></span><div><strong>{{ t(s.online ? 'online' : 'offline') }}</strong><p>{{ s.couple?.partner.display_name || t('bindFirst') }}</p></div><button @click="tab = 'me'">→</button></article>
      </section>
      <section v-if="tab === 'chat'"><ChatPanel :client="client" :t="t" /></section>
      <section v-if="tab === 'time'"><CalendarPanel :client="client" :t="t" /></section>
      <section v-if="tab === 'map'"><div class="heading"><h1>{{ t('map') }}</h1><p>{{ t('featurePending') }}</p></div><article class="card empty"><span class="empty-icon">⌖</span><p>{{ t('mapHelp') }}</p></article></section>
      <section v-if="tab === 'me'">
        <div class="heading"><h1>{{ t('me') }}</h1><p>CoupleLink · 1.0-r11</p></div>
        <article class="card"><h2>{{ t('account') }}</h2><p v-if="!isAndroid" class="subtle">{{ t('browserHelp') }}</p><p class="subtle">{{ t('migrationHelp') }}</p>
          <template v-if="!s.user"><div class="segmented"><button :class="{ active: !register }" @click="register = false">{{ t('login') }}</button><button :class="{ active: register }" @click="register = true">{{ t('register') }}</button></div><form @submit.prevent="run(login)"><label>{{ t('server') }}<input v-model="address" inputmode="url" autocomplete="url" required></label><p v-if="address.startsWith('http:')" class="warning">{{ t('httpWarning') }}</p><button type="button" class="outline" :disabled="pending" @click="run(async () => { feedback = t('testOk') + ' · ' + await client.test(address) })">{{ t('test') }}</button><p class="subtle">{{ t('addressHelp') }}</p><label>{{ t('username') }}<input v-model="username" autocomplete="username" pattern="[A-Za-z0-9_-]{3,32}" required></label><label v-if="register">{{ t('nickname') }}<input v-model="nickname" maxlength="80" required></label><label>{{ t('password') }}<input type="password" v-model="password" autocomplete="current-password" minlength="8" maxlength="128" required></label><button class="primary wide" :disabled="pending">{{ t(pending ? 'busy' : register ? 'register' : 'login') }}</button></form></template>
          <template v-else><div class="profile"><div class="avatar">{{ s.user.display_name.slice(0, 1) }}</div><div><h3>{{ s.user.display_name }}</h3><p>@{{ s.user.username }}</p></div></div><template v-if="!s.couple"><button class="outline wide" :disabled="pending" @click="run(() => client.invitation())">{{ t('createInvite') }}</button><output v-if="s.invitation" class="invite">{{ s.invitation }}</output><form @submit.prevent="run(() => client.join(invite, startDate))"><label>{{ t('invitation') }}<input v-model="invite" required></label><label>{{ t('date') }}<input type="date" v-model="startDate" required></label><button class="primary wide" :disabled="pending">{{ t('join') }}</button></form></template><template v-else><p>♡ {{ s.couple.partner.display_name }}</p><button class="outline" :disabled="pending" @click="run(confirmUnbind)">{{ t('unbind') }}</button></template><button class="text-button" :disabled="pending" @click="run(() => client.logout())">{{ t('logout') }}</button></template>
        </article>
        <article class="card"><h2>{{ t('notifications') }}</h2><p class="background-status" role="status">{{ t(s.background ? 'backgroundRunning' : 'backgroundStopped') }}</p><h3>{{ t('background') }}</h3><p class="subtle">{{ t('backgroundHelp') }}</p><button v-if="!s.background" class="primary wide" :disabled="pending" @click="run(permission)">{{ t('enableReminders') }}</button><div class="button-grid"><button class="outline" @click="run(notifyTest)">{{ t('notificationTest') }}</button><button class="outline" @click="run(() => feedbackTest(false))">{{ t('vibrationTest') }}</button><button class="outline" @click="run(() => feedbackTest(true))">{{ t('soundTest') }}</button></div><button class="text-button" @click="run(async () => { if(isAndroid) await native.settings(); else feedback = t('android_only') })">{{ t('systemSettings') }} ↗</button><button class="outline wide" @click="run(refreshDiagnostics)">{{ t('diagnostics') }}</button>
<details v-if="diagnostics" class="diagnostics"><summary>{{ t('technicalDiagnostics') }}</summary>
  <p class="subtle">{{ diagnostics.manufacturer }} {{ diagnostics.model }} · Android API {{ diagnostics.sdk }}</p>
  <dl><dt>{{ t('notificationPermission') }}</dt><dd>{{ t(diagnostics.notificationsEnabled ? 'enabled' : 'blocked') }}</dd><dt>{{ t('vibrationHardware') }}</dt><dd>{{ t(diagnostics.hasVibrator ? 'available' : 'unsupported') }}</dd><dt>{{ t('vibrationOperation') }}</dt><dd>{{ t(diagnostics.vibrationOperation === 'allowed' ? 'enabled' : diagnostics.vibrationOperation === 'blocked' ? 'blocked' : 'unknown') }}</dd><dt>{{ t('hapticFeedback') }}</dt><dd>{{ t(diagnostics.hapticFeedbackEnabled ? 'enabled' : 'disabled') }}</dd><dt>{{ t('notificationVibration') }}</dt><dd>{{ t(diagnostics.notificationVibration ? 'enabled' : 'disabled') }}</dd><dt>{{ t('vibrationMethod') }}</dt><dd>{{ t(diagnostics.feedbackMethod === 'touch' ? 'hapticFeedback' : diagnostics.feedbackMethod === 'notification' ? 'notificationVibration' : 'methodDisabled') }}</dd><dt>{{ t('soundMode') }}</dt><dd>{{ t(diagnostics.ringerMode === 0 ? 'silent' : diagnostics.ringerMode === 1 ? 'vibrationMode' : 'normalMode') }}</dd><dt>{{ t('dnd') }}</dt><dd>{{ t(diagnostics.interruptionFilter === 1 ? 'disabled' : diagnostics.interruptionFilter === 0 ? 'unknown' : 'enabled') }}</dd><dt>{{ t('background') }}</dt><dd>{{ t(diagnostics.running ? 'running' : 'stopped') }}</dd></dl>
  <p v-if="diagnostics.feedbackMethod === 'disabled'" class="subtle">{{ t('vibration_disabled') }}</p>
  <p v-else class="subtle">{{ t('hapticHelp') }}</p>
  <p class="subtle">{{ t('headsUpHelp') }}</p><details><summary>{{ t('diagnosticReport') }}</summary><textarea readonly rows="10" :value="JSON.stringify(diagnostics, null, 2)" :aria-label="t('diagnosticReport')"></textarea></details></details>
</article>
        <article class="card"><h2>{{ t('addresses') }}</h2><label>{{ t('server') }}<input v-model="address" inputmode="url"></label><p v-if="address.startsWith('http:')" class="warning">{{ t('httpWarning') }}</p><div class="button-grid"><button class="outline" @click="run(async () => { feedback = t('testOk') + ' · ' + await client.test(address) })">{{ t('test') }}</button><button class="outline" @click="run(async () => { const value = normalizeOrigin(address); if(!addresses.includes(value)) addresses.push(value) })">{{ t('addAddress') }}</button></div><p class="subtle">{{ t('manualAddresses') }}</p><div v-for="value in addresses" :key="value" class="address-row"><button :disabled="pending || value === s.origin" @click="run(() => selectServer(value))">{{ value }}<small v-if="value === s.origin">{{ t('selected') }}</small></button><button @click="addresses = addresses.filter(x => x !== value)" :aria-label="t('remove')">×</button></div><button class="primary wide" :disabled="pending" @click="run(() => selectServer(address))">{{ t('save') }}</button></article>
        <article class="card"><h2>{{ t('appearance') }}</h2><div class="themes"><button v-for="name in ['sakura', 'lavender', 'midnight']" :key="name" :class="{ active: theme === name }" @click="theme = name">{{ t(name) }}</button></div><label>{{ t('language') }}<select v-model="locale" :aria-label="t('language')"><option value="zh">简体中文</option><option value="en">English</option></select></label><details><summary>{{ t('licenses') }}</summary><pre class="licenses">{{ notices }}</pre></details></article>
      </section>
    </main>
    <nav :aria-label="t('space')"><button v-for="name in ['home', 'map', 'chat', 'time', 'me']" :key="name" :class="{ active: tab === name }" @click="tab = name"><span class="nav-icon">{{ icons[name] }}<i v-if="name === 'chat' && s.unread" class="unread" :aria-label="t('newMessage')"></i></span>{{ t(name) }}</button></nav>
    <div v-if="s.incoming" class="modal-backdrop"><section class="interaction-dialog" role="dialog" aria-modal="true" :aria-label="t('interactionTitle')"><p>{{ t('interactionTitle') }}</p><div class="heart">{{ actionIcons[s.incoming] }}</div><h1>{{ t(s.incoming) }}</h1><button class="primary wide" autofocus @click="s.incoming = ''">{{ t('close') }}</button></section></div>
  </div>
</template>
