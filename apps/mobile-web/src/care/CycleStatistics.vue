<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { errorCode, type Client } from '../core';

const props = defineProps<{ client: Client; t: (key: string) => string }>();

// The server sends snake_case codes; text() falls back to server_error for unknown keys.
const ERR: Record<string, string> = { partner_no_share: 'partnerNoShare', cycle_overlap: 'cycleOverlap', care_limit: 'careLimit', conflict: 'conflict', upgrade_required: 'upgrade_required', relationship_changed: 'relationship_changed', not_bound: 'not_bound', invalid_input: 'invalid_input' };
const errorText = (code: string) => ERR[code] || code;

const busy = ref(false), error = ref('');
const statistics = computed(() => props.client.care.statistics);
const cycles = computed(() => statistics.value?.cycles ?? []);

async function load() {
  if (busy.value) return;
  if (!props.client.state.couple || !props.client.care.supported) return;
  busy.value = true; error.value = '';
  try { await props.client.loadCareStatistics(); }
  catch (e) { error.value = errorCode(e); }
  finally { busy.value = false; }
}
onMounted(() => { void load(); });
</script>

<template>
  <div class="stats-page">
    <p v-if="error" class="banner error" role="alert">{{ t(errorText(error)) }}</p>
    <article class="card">
      <div class="stats-head">
        <h2>{{ t('cycleStats') }}</h2>
        <button class="stats-refresh" type="button" :disabled="busy" @click="load">{{ busy ? t('busy') : t('refresh') }}</button>
      </div>
      <p v-if="!cycles.length" class="empty">{{ t('noCycleData') }}</p>
      <template v-else>
        <div class="stats-grid">
          <div class="stats-tile"><span>{{ t('averageCycle') }}</span><strong>{{ statistics ? statistics.average_cycle_days : 0 }} {{ t('dayUnit') }}</strong></div>
          <div class="stats-tile"><span>{{ t('averagePeriod') }}</span><strong>{{ statistics ? statistics.average_period_days : 0 }} {{ t('dayUnit') }}</strong></div>
        </div>
        <div v-if="statistics && statistics.predicted_start_date" class="stats-predict">
          <p class="stats-label">{{ t('predicted') }}</p>
          <p class="stats-date">{{ statistics.predicted_start_date }}<template v-if="statistics.predicted_end_date"> → {{ statistics.predicted_end_date }}</template></p>
          <p v-if="statistics.days_until_next !== null" class="subtle">{{ t('daysUntilNext') }} {{ statistics.days_until_next }} {{ t('dayUnit') }}</p>
          <p class="stats-estimate">{{ t('estimateOnly') }}</p>
        </div>
      </template>
    </article>
    <article class="card">
      <h2>{{ t('cycleHistory') }}</h2>
      <p v-if="!cycles.length" class="empty">{{ t('noCycleData') }}</p>
      <ul v-else class="stats-list">
        <li v-for="entry in cycles" :key="entry.start_date" class="stats-row">
          <span class="stats-dates">{{ entry.start_date }} → {{ entry.end_date || '—' }}</span>
          <span class="stats-length">{{ entry.length_days === null ? '—' : entry.length_days }} <template v-if="entry.length_days !== null">{{ t('dayUnit') }}</template></span>
        </li>
      </ul>
    </article>
  </div>
</template>

<style scoped>
.stats-page { width: 100%; max-width: 100%; min-width: 0; overflow-wrap: anywhere; }
button { min-height: 44px; }
.stats-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
.stats-head h2 { margin: 0; }
.stats-refresh { border: 1px solid var(--accent); background: transparent; color: var(--accent); border-radius: 999px; padding: 10px 16px; font-size: 13px; }
.stats-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; margin-top: 14px; }
.stats-tile { background: var(--soft); border-radius: 16px; padding: 12px 10px; display: flex; flex-direction: column; gap: 6px; min-width: 0; text-align: center; }
.stats-tile span { color: var(--muted); font-size: 12px; line-height: 1.2; }
.stats-tile strong { font-size: 18px; color: var(--accent); }
.stats-predict { margin-top: 16px; padding-top: 14px; border-top: 1px solid var(--line); }
.stats-label { font-size: 13px; color: var(--muted); margin: 0 0 4px; }
.stats-date { font-size: 18px; font-weight: 650; margin: 0; }
.stats-estimate { font-size: 12px; color: var(--muted); margin: 8px 0 0; }
.stats-list { list-style: none; margin: 0; padding: 0; }
.stats-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 12px 0; border-bottom: 1px solid var(--line); font-size: 14px; }
.stats-row:last-child { border-bottom: none; }
.stats-dates { min-width: 0; overflow-wrap: anywhere; }
.stats-length { color: var(--accent); font-size: 13px; white-space: nowrap; }
</style>
