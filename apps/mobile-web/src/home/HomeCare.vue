<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import type { Client } from '../core';

const props = defineProps<{ client: Client; t: (key: string) => string }>();
const emit = defineEmits<{ open: [] }>();

const loaded = ref(false);

// The first screen must never break on a care request: a failure keeps the card on
// the empty state and the care center renders its own error handling.
async function loadCare() {
  if (!props.client.state.couple || !props.client.care.supported) return;
  loaded.value = false;
  try { await props.client.loadCareDashboard(); } catch { /* empty state below */ }
  finally { loaded.value = true; }
}

onMounted(() => { void loadCare(); });
// The home section mounts before login restore finishes, so reload when pairing lands.
watch(() => [props.client.state.couple?.id, props.client.care.supported], () => { void loadCare(); });

const bound = computed(() => !!props.client.state.couple);
const supported = computed(() => props.client.care.supported);
const today = computed(() => props.client.care.dashboard?.today || null);

// strings.ts has no dedicated "period day" phrase, so the count reuses existing keys.
function periodLabel(day: number) { return props.t('todayCare') + ' · ' + day + ' ' + props.t('dayUnit'); }
</script>

<template>
  <article class="card home-section care-card">
    <button type="button" class="care-entry" @click="emit('open')">
      <span class="care-entry-ic"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 20.3S4.6 15.9 4.6 10.9A4.2 4.2 0 0 1 12 8.3a4.2 4.2 0 0 1 7.4 2.6c0 5-7.4 9.4-7.4 9.4z" /><path d="M9.5 12.5h5M12 10v5" /></svg></span>
      <span class="care-entry-text"><b>{{ t('careCenter') }}</b><small>{{ t('careCenterHint') }}</small></span>
      <i class="chev" aria-hidden="true">›</i>
    </button>
    <div class="care-today">
      <p class="care-today-title">{{ t('todayCare') }}</p>
      <p v-if="!bound" class="subtle care-empty">{{ t('notBound') }}</p>
      <p v-else-if="!supported" class="subtle care-empty">{{ t('careUnsupported') }}</p>
      <p v-else-if="loaded && !today" class="subtle care-empty">{{ t('todayCareEmpty') }}</p>
      <ul v-else-if="today" class="care-today-list">
        <li v-if="today.in_period" class="care-chip accent">{{ periodLabel(today.period_day) }}</li>
        <li class="care-chip">{{ today.has_log ? t('logSaved') : t('logToday') }}</li>
        <li class="care-chip">{{ t('careChecklist') }} · {{ today.open_tasks }}</li>
      </ul>
    </div>
  </article>
</template>

<style scoped>
.care-today{margin-top:14px;padding-top:14px;border-top:1px solid var(--line);min-width:0}
.care-today-title{margin:0 0 8px;font-size:12px;font-weight:700;letter-spacing:.1em;text-transform:uppercase;color:var(--muted)}
.care-empty{margin:0}
.care-today-list{display:flex;flex-wrap:wrap;gap:8px;margin:0;padding:0;list-style:none;min-width:0}
.care-chip{padding:6px 12px;border-radius:999px;background:var(--soft);color:var(--muted);font-size:12px;font-weight:600;overflow-wrap:anywhere;min-width:0}
.care-chip.accent{background:var(--soft);color:var(--accent)}
</style>
