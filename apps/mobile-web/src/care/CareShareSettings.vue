<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { errorCode, type CareScope, type Client } from '../core';
import { ymd } from './cycle';

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

const busy = ref(false), error = ref(''), notice = ref(''), pending = ref('');
const shares = computed(() => props.client.care.shares);
const bound = computed(() => !!props.client.state.couple);
const partner = computed(() => props.client.care.dashboard?.partner ?? null);
const partnerScopes = computed(() => {
  const value = partner.value;
  if (!value) return [];
  return SCOPES.filter(item => !!value.grants[item.scope]).map(item => item.scope);
});

function shareOf(scope: CareScope) { return shares.value.find(entry => entry.scope === scope) || null; }
function grantedAt(scope: CareScope) { const share = shareOf(scope); return share ? ymd(new Date(share.granted_at)) : ''; }
function scopeKey(scope: CareScope) { const item = SCOPES.find(entry => entry.scope === scope); return item ? item.key : 'shareHint'; }

async function run(fn: () => Promise<void>) {
  if (busy.value) return;
  busy.value = true; error.value = '';
  try { await fn(); }
  catch (e) { error.value = errorCode(e); }
  finally { busy.value = false; pending.value = ''; }
}
onMounted(() => {
  if (!bound.value || !props.client.care.supported) return;
  void run(async () => { await props.client.loadCareShares(); });
});

async function grant(scope: CareScope) {
  pending.value = scope;
  await run(async () => {
    await props.client.grantCareShare(scope);
    notice.value = 'saved';
  });
}
async function revoke(scope: CareScope) {
  const share = shareOf(scope);
  if (!share) return;
  pending.value = scope;
  await run(async () => {
    await props.client.revokeCareShare(share.id);
    notice.value = 'shareRevoked';
  });
}
</script>

<template>
  <div class="share-page">
    <p v-if="error" class="banner error" role="alert">{{ t(errorText(error)) }}</p>
    <p v-if="notice" class="banner" role="status">{{ t(notice) }}</p>
    <article v-if="!bound" class="card empty">{{ t('notBound') }}</article>
    <article v-else class="card">
      <h2>{{ t('shareSettings') }}</h2>
      <p class="subtle share-hint">{{ t('shareHint') }}</p>
      <ul class="share-list">
        <li v-for="item in SCOPES" :key="item.scope" class="share-row">
          <div class="share-text">
            <span class="share-name">{{ t(item.key) }}</span>
            <span v-if="shareOf(item.scope)" class="share-state">{{ grantedAt(item.scope) }}</span>
          </div>
          <button v-if="shareOf(item.scope)" class="share-revoke" type="button" :disabled="busy || pending === item.scope" @click="revoke(item.scope)">{{ busy && pending === item.scope ? t('busy') : t('revokeShare') }}</button>
          <button v-else class="share-grant" type="button" :disabled="busy || pending === item.scope" @click="grant(item.scope)">{{ busy && pending === item.scope ? t('busy') : t('grantShare') }}</button>
        </li>
      </ul>
      <div class="share-partner">
        <h3>{{ t('partnerCare') }}</h3>
        <p v-if="!partnerScopes.length" class="subtle">{{ t('partnerNoShare') }}</p>
        <ul v-else class="share-partner-list">
          <li v-for="scope in partnerScopes" :key="scope">✓ {{ t(scopeKey(scope)) }}</li>
        </ul>
      </div>
    </article>
  </div>
</template>

<style scoped>
.share-page { width: 100%; max-width: 100%; min-width: 0; overflow-wrap: anywhere; }
button { min-height: 44px; }
.share-hint { margin-top: 0; }
.share-list { list-style: none; margin: 12px 0 0; padding: 0; }
.share-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 12px 0; border-bottom: 1px solid var(--line); }
.share-row:last-child { border-bottom: none; }
.share-text { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.share-name { font-size: 14px; overflow-wrap: anywhere; }
.share-state { font-size: 11px; color: var(--muted); }
.share-grant, .share-revoke { flex: 0 0 auto; border-radius: 999px; padding: 10px 14px; font-size: 13px; }
.share-grant { border: 1px solid var(--accent); background: transparent; color: var(--accent); }
.share-revoke { border: 1px solid var(--line); background: transparent; color: var(--muted); }
.share-partner { margin-top: 18px; padding-top: 14px; border-top: 1px solid var(--line); }
.share-partner h3 { margin: 0 0 6px; font-size: 15px; }
.share-partner-list { list-style: none; margin: 6px 0 0; padding: 0; font-size: 13px; color: var(--ink); }
.share-partner-list li { padding: 4px 0; overflow-wrap: anywhere; }
</style>
