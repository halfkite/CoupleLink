<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import type { Anniversary, Client } from '../core';
import { anniversaryDays } from '../memories';

const props = defineProps<{ client: Client; t: (key: string) => string }>();
const emit = defineEmits<{ open: [target: 'time' | 'me'] }>();

// The relationship card and the anniversary list share one compact block: the first
// screen keeps the days count without spending a whole card on each piece of data.
const loaded = ref(false);

async function loadDates() {
  if (!props.client.state.couple || !props.client.memories.supported) { loaded.value = true; return; }
  try { await props.client.loadAnniversaries(); } catch { /* empty state below */ }
  finally { loaded.value = true; }
}

// The home section mounts before login restore finishes, so reload when pairing lands.
// Compare primitives: an array literal would differ on every evaluation and reload on
// each sync tick.
function scope() { return (props.client.state.couple?.id || '') + '|' + (props.client.memories.supported ? '1' : '0'); }
onMounted(() => { void loadDates(); });
watch(scope, () => { void loadDates(); });

const couple = computed(() => props.client.state.couple);
const days = computed(() => couple.value?.days_together ?? null);
const partnerName = computed(() => couple.value?.my_remark || couple.value?.partner?.display_name || '');
const supported = computed(() => props.client.memories.supported);
const upcoming = computed(() => props.client.memories.anniversaries
  .map(anniversary => {
    try { return { anniversary, days: anniversaryDays(anniversary) }; }
    catch { return null; }
  })
  .filter((entry): entry is { anniversary: Anniversary; days: number } => !!entry)
  .sort((a, b) => Math.abs(a.days) - Math.abs(b.days) || a.anniversary.date.localeCompare(b.anniversary.date))
  .slice(0, 3));
const datesHint = computed(() => !couple.value ? 'notBound' : !supported.value ? 'upgrade_required' : 'noAnniversaries');

function countdown(days: number) { return days === 0 ? props.t('today') : Math.abs(days) + ' ' + props.t(days > 0 ? 'daysUntil' : 'daysAgo'); }
</script>

<template>
  <article class="card bond-card">
    <button type="button" class="bond-row" :aria-label="t(couple ? 'editStartDate' : 'connectFirst')" @click="emit('open', 'me')">
      <span class="bond-avatars" aria-hidden="true">
        <span class="bond-avatar"><img v-if="couple?.my_avatar && client.imageURL(couple.my_avatar)" :src="client.imageURL(couple.my_avatar)" alt="" /><template v-else>{{ client.state.user?.display_name?.slice(0, 1) || '♡' }}</template></span>
        <i>♥</i>
        <span class="bond-avatar partner"><img v-if="couple?.partner_avatar && client.imageURL(couple.partner_avatar)" :src="client.imageURL(couple.partner_avatar)" alt="" /><template v-else>{{ partnerName?.slice(0, 1) || '♡' }}</template></span>
      </span>
      <span class="bond-days">
        <strong>{{ days ?? '—' }}</strong><small>{{ t('dayUnit') }}</small>
      </span>
      <span class="bond-side">
        <b>{{ couple ? t('days') : t(client.state.user ? 'bindFirst' : 'connectFirst') }}</b>
        <small>{{ couple ? couple.start_at : t('connectFirst') }}</small>
      </span>
    </button>

    <div class="bond-dates">
      <div class="bond-dates-head">
        <h2>{{ t('commonAnniversaries') }}</h2>
        <button type="button" class="text-button" @click="emit('open', 'time')">{{ t('manage') }} ›</button>
      </div>
      <p v-if="!upcoming.length && (loaded || !couple || !supported)" class="subtle bond-empty">{{ t(datesHint) }}</p>
      <button v-for="entry in upcoming" :key="entry.anniversary.id" type="button" class="bond-date-row" @click="emit('open', 'time')">
        <span class="bond-date-text"><b>{{ entry.anniversary.title }}</b><small>{{ entry.anniversary.date }}</small></span>
        <strong class="bond-date-count">{{ countdown(entry.days) }}</strong>
      </button>
    </div>
  </article>
</template>

<style scoped>
.bond-card{display:flex;flex-direction:column;gap:0;min-width:0;padding:14px 16px}
.bond-row{display:flex;align-items:center;gap:12px;width:100%;min-width:0;padding:0;background:none;border:0;text-align:left;color:var(--ink);min-height:52px}
.bond-avatars{display:flex;align-items:center;gap:4px;flex-shrink:0}
.bond-avatar{position:relative;display:grid;place-items:center;width:34px;height:34px;border-radius:50%;background:var(--soft);color:var(--accent);font-size:14px;overflow:hidden}
.bond-avatar img{position:absolute;inset:0;width:100%;height:100%;object-fit:cover;border-radius:50%}
.bond-avatars i{font-style:normal;color:var(--accent);font-size:11px}
.bond-days{display:flex;align-items:baseline;gap:3px;flex-shrink:0}
.bond-days strong{font-size:30px;line-height:1;font-weight:750;letter-spacing:-1px;color:var(--accent);font-variant-numeric:tabular-nums}
.bond-days small{font-size:11px;color:var(--muted)}
.bond-side{display:flex;flex-direction:column;gap:2px;min-width:0;margin-left:auto;text-align:right}
.bond-side b{font-size:12.5px;font-weight:650;color:var(--muted);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.bond-side small{font-size:11px;color:var(--muted);font-variant-numeric:tabular-nums}
.bond-dates{margin-top:12px;padding-top:12px;border-top:1px solid var(--line);min-width:0}
.bond-dates-head{display:flex;align-items:center;justify-content:space-between;gap:10px}
.bond-dates-head h2{font-size:13px;margin:0;font-weight:700;letter-spacing:.06em;color:var(--muted)}
.bond-dates-head .text-button{font-size:12px;min-height:32px;padding:4px 0}
.bond-empty{margin:8px 0 0}
.bond-date-row{display:flex;align-items:center;justify-content:space-between;gap:10px;width:100%;padding:8px 0;background:none;border:0;border-bottom:1px solid var(--soft);text-align:left;color:var(--ink);min-height:40px}
.bond-date-row:last-child{border-bottom:0;padding-bottom:0}
.bond-date-text{display:flex;flex-direction:column;gap:1px;min-width:0;flex:1}
.bond-date-text b{font-size:13.5px;font-weight:650;overflow-wrap:anywhere}
.bond-date-text small{font-size:11px;color:var(--muted);font-variant-numeric:tabular-nums}
.bond-date-count{font-size:12.5px;color:var(--accent);white-space:nowrap;flex-shrink:0}
</style>
