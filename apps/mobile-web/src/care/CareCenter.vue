<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { errorCode, type Client } from '../core';
import CarePrivacy from './CarePrivacy.vue';
import CareReminders from './CareReminders.vue';
import CareShareSettings from './CareShareSettings.vue';
import CareTasks from './CareTasks.vue';
import CycleCalendar from './CycleCalendar.vue';
import CycleRecord from './CycleRecord.vue';
import CycleStatistics from './CycleStatistics.vue';

const props = defineProps<{ client: Client; t: (key: string) => string }>();
const emit = defineEmits<{ (e: 'close'): void }>();

type Page = 'home' | 'calendar' | 'record' | 'stats' | 'tasks' | 'share' | 'reminders' | 'privacy';

// The server sends snake_case codes; text() falls back to server_error for unknown keys.
const ERR: Record<string, string> = { partner_no_share: 'partnerNoShare', cycle_overlap: 'cycleOverlap', care_limit: 'careLimit', conflict: 'conflict', upgrade_required: 'upgrade_required', relationship_changed: 'relationship_changed', not_bound: 'not_bound', invalid_input: 'invalid_input' };
const errorText = (code: string) => ERR[code] || code;

const page = ref<Page>('home');
const busy = ref(false), error = ref('');

const bound = computed(() => !!props.client.state.couple);
const supported = computed(() => props.client.care.supported);
const dashboard = computed(() => props.client.care.dashboard);
const today = computed(() => dashboard.value?.today ?? null);
const prediction = computed(() => dashboard.value?.prediction ?? null);
const partner = computed(() => dashboard.value?.partner ?? null);
const partnerShared = computed(() => {
  const value = partner.value;
  if (!value) return false;
  if (value.summary) return true;
  return Object.values(value.grants || {}).some(Boolean);
});

const pages: Array<{ page: Page; key: string; icon: string }> = [
  { page: 'home', key: 'careHome', icon: '⌂' },
  { page: 'calendar', key: 'cycleCalendar', icon: '▦' },
  { page: 'record', key: 'dailyLog', icon: '✎' },
  { page: 'stats', key: 'cycleStats', icon: '▤' },
  { page: 'tasks', key: 'careChecklist', icon: '✓' },
  { page: 'share', key: 'shareSettings', icon: '⇄' },
  { page: 'reminders', key: 'reminderSettings', icon: '⏰' },
  { page: 'privacy', key: 'privacy', icon: '🔒' },
];
const titleKey = computed(() => (pages.find(item => item.page === page.value) || pages[0]).key);

async function load() {
  if (busy.value || !bound.value || !supported.value) return;
  busy.value = true; error.value = '';
  try { await Promise.all([props.client.loadCareDashboard(), props.client.loadCareProfile()]); }
  catch (e) { error.value = errorCode(e); }
  finally { busy.value = false; }
}
onMounted(() => { void load(); });

function go(next: Page) { page.value = next; if (next === 'home') void load(); }
function backHome() { page.value = 'home'; void load(); }
</script>

<template>
  <div class="care-page">
    <div class="care-top">
      <button class="care-back" type="button" @click="emit('close')">‹ {{ t('backHome') }}</button>
      <button class="care-refresh" type="button" :disabled="busy || !bound || !supported" @click="load">{{ busy ? t('busy') : t('refresh') }}</button>
    </div>
    <h1 class="care-title">{{ t('careCenter') }}</h1>
    <p v-if="error" class="banner error" role="alert">{{ t(errorText(error)) }}</p>
    <article v-if="!bound" class="card empty">{{ t('notBound') }}</article>
    <article v-else-if="!supported" class="card empty">{{ t('careUnsupported') }}</article>
    <template v-else>
      <template v-if="page === 'home'">
        <p class="subtle">{{ t('careCenterHint') }}</p>
        <article class="card">
          <h2>{{ t('todayCare') }}</h2>
          <div class="care-tiles">
            <div class="care-tile"><span>{{ t('cycleCalendar') }}</span><strong>{{ today && today.in_period ? today.period_day + ' ' + t('dayUnit') : '—' }}</strong></div>
            <div class="care-tile"><span>{{ t('dailyLog') }}</span><strong>{{ today && today.has_log ? t('saved') : t('logToday') }}</strong></div>
            <div class="care-tile"><span>{{ t('careChecklist') }}</span><strong>{{ today ? today.open_tasks : 0 }}</strong></div>
          </div>
        </article>
        <article class="card">
          <h2>{{ t('predicted') }}</h2>
          <template v-if="prediction && prediction.start_date">
            <p class="care-big">{{ prediction.start_date }}</p>
            <p v-if="prediction.days_until !== null" class="subtle">{{ t('daysUntilNext') }} {{ prediction.days_until }} {{ t('dayUnit') }}</p>
            <p class="care-estimate">{{ t('estimateOnly') }}</p>
          </template>
          <p v-else class="subtle">{{ t('noCycleData') }}</p>
        </article>
        <article class="card">
          <h2>{{ t('partnerCare') }}</h2>
          <p v-if="!partnerShared" class="subtle">{{ t('partnerNoShare') }}</p>
          <template v-else>
            <p v-if="partner" class="care-big">{{ partner.display_name }}</p>
            <p v-if="partner && partner.summary && partner.summary.predicted_start_date" class="subtle">{{ t('predicted') }} {{ partner.summary.predicted_start_date }}<template v-if="partner.summary.days_until_next !== null"> · {{ t('daysUntilNext') }} {{ partner.summary.days_until_next }} {{ t('dayUnit') }}</template></p>
            <p v-else class="subtle">{{ t('partnerNoShare') }}</p>
            <p class="care-estimate">{{ t('estimateOnly') }}</p>
          </template>
        </article>
        <div class="care-entries">
          <button v-for="entry in pages" :key="entry.page" type="button" class="care-entry" :class="{ active: page === entry.page }" :aria-current="page === entry.page ? 'page' : undefined" @click="go(entry.page)">
            <span class="care-icon" aria-hidden="true">{{ entry.icon }}</span>
            <span class="care-label">{{ t(entry.key) }}</span>
          </button>
        </div>
      </template>
      <template v-else>
        <div class="care-subhead">
          <button class="care-back" type="button" @click="backHome">‹ {{ t('careHome') }}</button>
          <strong class="care-subtitle">{{ t(titleKey) }}</strong>
        </div>
        <CycleCalendar v-if="page === 'calendar'" :client="client" :t="t" />
        <CycleRecord v-else-if="page === 'record'" :client="client" :t="t" />
        <CycleStatistics v-else-if="page === 'stats'" :client="client" :t="t" />
        <CareTasks v-else-if="page === 'tasks'" :client="client" :t="t" />
        <CareShareSettings v-else-if="page === 'share'" :client="client" :t="t" />
        <CareReminders v-else-if="page === 'reminders'" :client="client" :t="t" />
        <CarePrivacy v-else-if="page === 'privacy'" :client="client" :t="t" />
      </template>
    </template>
  </div>
</template>

<style scoped>
.care-page { width: 100%; max-width: 100%; min-width: 0; box-sizing: border-box; overflow-wrap: anywhere; }
.care-top { display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; }
.care-back, .care-refresh { min-height: 44px; border: 1px solid var(--line); background: var(--soft); color: var(--ink); border-radius: 999px; padding: 10px 14px; font-size: 13px; }
.care-refresh { color: var(--accent); border-color: var(--accent); background: transparent; }
.care-title { font-size: 24px; margin: 10px 0 4px; }
.care-tiles { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
.care-tile { background: var(--soft); border-radius: 16px; padding: 10px 8px; display: flex; flex-direction: column; gap: 6px; min-width: 0; text-align: center; }
.care-tile span { color: var(--muted); font-size: 12px; line-height: 1.2; }
.care-tile strong { font-size: 15px; color: var(--accent); overflow-wrap: anywhere; }
.care-big { font-size: 20px; font-weight: 650; margin: 4px 0; }
.care-estimate { font-size: 12px; color: var(--muted); margin: 8px 0 0; }
.care-entries { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; margin: 14px 0 20px; }
.care-entry { min-height: 84px; min-width: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 6px; padding: 8px 4px; border-radius: 18px; border: 1px solid var(--line); background: var(--card); color: var(--ink); font-size: 11px; line-height: 1.2; }
.care-entry.active { border-color: var(--accent); background: var(--soft); }
.care-icon { font-size: 20px; line-height: 1; color: var(--accent); }
.care-label { text-align: center; overflow-wrap: anywhere; }
.care-subhead { display: flex; align-items: center; gap: 10px; margin: 8px 0 12px; flex-wrap: wrap; }
.care-subtitle { font-size: 17px; }
@media (max-width: 359px) {
  .care-entries { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .care-tiles { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .care-tiles .care-tile:last-child { grid-column: 1 / -1; }
}
</style>
