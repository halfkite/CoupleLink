<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import type { Anniversary, Client } from '../core';
import { anniversaryDays } from '../memories';

const props = defineProps<{ client: Client; t: (key: string) => string }>();
const emit = defineEmits<{ open: [] }>();

const loaded = ref(false);

// Dates are a basic couple capability, but the preview stays silent on a server that
// has no memories support, so the home page never shows a request error.
async function loadDates() {
  if (!props.client.state.couple || !props.client.memories.supported) return;
  loaded.value = false;
  try { await props.client.loadAnniversaries(); } catch { /* empty state below */ }
  finally { loaded.value = true; }
}

onMounted(() => { void loadDates(); });
// The home section mounts before login restore finishes, so reload when pairing lands.
watch(() => [props.client.state.couple?.id, props.client.memories.supported], () => { void loadDates(); });

const bound = computed(() => !!props.client.state.couple);
const supported = computed(() => props.client.memories.supported);
// Closest dates first, upcoming or past; the countdown itself tells the direction.
// A malformed timezone must not break the first screen, so such a row is dropped.
const upcoming = computed(() => props.client.memories.anniversaries
  .map(anniversary => {
    try { return { anniversary, days: anniversaryDays(anniversary) }; }
    catch { return null; }
  })
  .filter((entry): entry is { anniversary: Anniversary; days: number } => !!entry)
  .sort((a, b) => Math.abs(a.days) - Math.abs(b.days) || a.anniversary.date.localeCompare(b.anniversary.date))
  .slice(0, 3));
const emptyKey = computed(() => !bound.value ? 'notBound' : !supported.value ? 'upgrade_required' : 'noAnniversaries');

function countdown(days: number) { return days === 0 ? props.t('today') : Math.abs(days) + ' ' + props.t(days > 0 ? 'daysUntil' : 'daysAgo'); }
</script>

<template>
  <article class="card home-section home-dates">
    <div class="home-section-head">
      <h2>{{ t('commonAnniversaries') }}</h2>
      <button type="button" class="text-button" @click="emit('open')">{{ t('manage') }} ›</button>
    </div>
    <p v-if="!upcoming.length && (loaded || !bound || !supported)" class="subtle home-empty">{{ t(emptyKey) }}</p>
    <div v-else-if="upcoming.length" class="home-dates-list">
      <div v-for="entry in upcoming" :key="entry.anniversary.id" class="home-dates-row">
        <span class="home-dates-text"><b>{{ entry.anniversary.title }}</b><small>{{ entry.anniversary.date }}</small></span>
        <strong class="home-dates-count">{{ countdown(entry.days) }}</strong>
      </div>
    </div>
  </article>
</template>

<style scoped>
.home-dates-text{display:flex;flex-direction:column;gap:2px;min-width:0;flex:1}
.home-dates-text b{font-size:14px;font-weight:650;overflow-wrap:anywhere}
.home-dates-text small{font-size:11.5px;color:var(--muted);font-variant-numeric:tabular-nums}
.home-dates-count{font-size:13px;color:var(--accent);white-space:nowrap;flex-shrink:0}
</style>
