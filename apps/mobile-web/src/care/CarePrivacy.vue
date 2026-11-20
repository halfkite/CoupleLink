<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { errorCode, type CareScope, type Client } from '../core';

const props = defineProps<{ client: Client; t: (key: string) => string }>();

// The server sends snake_case codes; text() falls back to server_error for unknown keys.
const ERR: Record<string, string> = { partner_no_share: 'partnerNoShare', cycle_overlap: 'cycleOverlap', care_limit: 'careLimit', conflict: 'conflict', upgrade_required: 'upgrade_required', relationship_changed: 'relationship_changed', not_bound: 'not_bound', invalid_input: 'invalid_input' };
const errorText = (code: string) => ERR[code] || code;

const SCOPES: Array<{ scope: CareScope; key: string }> = [
  { scope: 'cycle_summary', key: 'shareCycleSummary' },
  { scope: 'period_dates', key: 'sharePeriodDates' },
  { scope: 'daily_logs', key: 'shareDailyLogs' },
  { scope: 'care_preferences', key: 'sharePreferences' },
];

const busy = ref(false), error = ref(''), notice = ref(''), confirming = ref(false);
const bound = computed(() => !!props.client.state.couple);
const shares = computed(() => props.client.care.shares);
const periods = computed(() => props.client.care.periods);
const logs = computed(() => props.client.care.logs);

async function run(fn: () => Promise<void>) {
  if (busy.value) return;
  busy.value = true; error.value = '';
  try { await fn(); }
  catch (e) { error.value = errorCode(e); }
  finally { busy.value = false; }
}
onMounted(() => {
  if (!bound.value || !props.client.care.supported) return;
  void run(async () => { await Promise.all([props.client.loadCareShares(), props.client.loadPeriods(), props.client.loadLogs()]); });
});

async function revokeAll() {
  await run(async () => {
    for (const share of [...props.client.care.shares]) await props.client.revokeCareShare(share.id);
    await props.client.loadCareShares();
    notice.value = 'shareRevoked';
  });
}
async function deleteAll() {
  confirming.value = false;
  await run(async () => {
    for (const period of [...props.client.care.periods]) await props.client.deletePeriod(period.id);
    for (const log of [...props.client.care.logs]) await props.client.deleteLog(log.id);
    await Promise.all([props.client.loadPeriods(), props.client.loadLogs()]);
    notice.value = 'deleteHealthDataDone';
  });
}
</script>

<template>
  <div class="priv-page">
    <p v-if="error" class="banner error" role="alert">{{ t(errorText(error)) }}</p>
    <p v-if="notice" class="banner" role="status">{{ t(notice) }}</p>
    <article v-if="!bound" class="card empty">{{ t('notBound') }}</article>
    <template v-else>
      <article class="card">
        <h2>{{ t('privacy') }}</h2>
        <p class="subtle">{{ t('shareHint') }}</p>
        <ul class="priv-list">
          <li v-for="item in SCOPES" :key="item.scope" class="priv-row">{{ t(item.key) }}</li>
        </ul>
        <p class="priv-note">{{ t('privacyHint') }}</p>
      </article>
      <article class="card">
        <h2>{{ t('revokeAllShares') }}</h2>
        <button class="priv-danger" type="button" :disabled="busy || !shares.length" @click="revokeAll">{{ busy ? t('busy') : t('revokeAllShares') }}</button>
      </article>
      <article class="card">
        <h2>{{ t('deleteHealthData') }}</h2>
        <p class="subtle">{{ t('privacyHint') }}</p>
        <button class="priv-danger" type="button" :disabled="busy || (!periods.length && !logs.length)" @click="confirming = true">{{ busy ? t('busy') : t('deleteHealthData') }}</button>
      </article>
    </template>

    <div v-if="confirming" class="modal-backdrop" @click.self="confirming = false">
      <section class="confirm-card" role="dialog" aria-modal="true">
        <p>{{ t('deleteHealthDataConfirm') }}</p>
        <div class="confirm-actions">
          <button class="outline pill" type="button" @click="confirming = false">{{ t('cancel') }}</button>
          <button class="primary pill" type="button" :disabled="busy" @click="deleteAll">{{ busy ? t('busy') : t('deleteHealthData') }}</button>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.priv-page { width: 100%; max-width: 100%; min-width: 0; overflow-wrap: anywhere; }
button { min-height: 44px; }
.priv-list { list-style: none; margin: 12px 0 0; padding: 0; }
.priv-row { padding: 10px 14px; margin-bottom: 8px; border-radius: 14px; background: var(--soft); color: var(--ink); font-size: 14px; overflow-wrap: anywhere; }
.priv-note { margin: 12px 0 0; padding: 12px 14px; border-radius: 14px; border-left: 3px solid var(--accent); background: var(--soft); color: var(--ink); font-size: 13px; line-height: 1.5; }
.priv-danger { width: 100%; border: 1px solid var(--accent); background: transparent; color: var(--accent); border-radius: 999px; padding: 12px 16px; font-size: 14px; }
</style>
