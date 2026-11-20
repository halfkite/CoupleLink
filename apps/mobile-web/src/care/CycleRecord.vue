<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { errorCode, type Client, type CycleLog } from '../core';
import { MOODS, SYMPTOMS, validateLog, ymd } from './cycle';

const props = defineProps<{ client: Client; t: (key: string) => string }>();

// The server sends snake_case codes; text() falls back to server_error for unknown keys.
const ERR: Record<string, string> = { partner_no_share: 'partnerNoShare', cycle_overlap: 'cycleOverlap', care_limit: 'careLimit', conflict: 'conflict', upgrade_required: 'upgrade_required', relationship_changed: 'relationship_changed', not_bound: 'not_bound', invalid_input: 'invalid_input' };
const errorText = (code: string) => ERR[code] || code;

const MOOD_KEY: Record<string, string> = { happy: 'moodHappy', calm: 'moodCalm', tired: 'moodTired', sad: 'moodSad', irritable: 'moodIrritable', anxious: 'moodAnxious' };
const SYMPTOM_KEY: Record<string, string> = { cramps: 'symptomCramps', headache: 'symptomHeadache', fatigue: 'symptomFatigue', bloating: 'symptomBloating', backache: 'symptomBackache', insomnia: 'symptomInsomnia' };
const moodOptions = MOODS.map(value => ({ value, key: MOOD_KEY[value] }));
const symptomOptions = SYMPTOMS.map(value => ({ value, key: SYMPTOM_KEY[value] }));

type Draft = { pain_level: number; mood: string; symptoms: string[]; note: string };

const date = ref(ymd(new Date()));
const form = ref<Draft>({ pain_level: 0, mood: '', symptoms: [], note: '' });
const busy = ref(false), error = ref(''), notice = ref(''), confirming = ref<CycleLog | null>(null);
const existing = computed(() => props.client.care.logs.find(log => log.log_date === date.value) || null);

function apply() {
  const log = existing.value;
  form.value = log
    ? { pain_level: log.pain_level, mood: log.mood || '', symptoms: [...log.symptoms], note: log.note }
    : { pain_level: 0, mood: '', symptoms: [], note: '' };
  notice.value = '';
}
watch([date, () => props.client.care.logs], apply, { immediate: true });

async function run(fn: () => Promise<void>) {
  if (busy.value) return;
  busy.value = true; error.value = '';
  try { await fn(); }
  catch (e) { error.value = errorCode(e); }
  finally { busy.value = false; }
}
onMounted(() => {
  if (!props.client.state.couple || !props.client.care.supported) return;
  void run(async () => { await props.client.loadLogs(); });
});

function setToday() { date.value = ymd(new Date()); }
function toggleSymptom(value: string) {
  const list = form.value.symptoms;
  const index = list.indexOf(value);
  if (index >= 0) list.splice(index, 1); else list.push(value);
}
function clampPain(): number {
  const value = Math.trunc(Number(form.value.pain_level));
  if (!Number.isFinite(value)) return 0;
  return Math.min(10, Math.max(0, value));
}
async function save() {
  await run(async () => {
    const draft = { log_date: date.value, pain_level: clampPain(), mood: form.value.mood, symptoms: [...form.value.symptoms], note: form.value.note };
    const invalid = validateLog(draft, ymd(new Date()));
    if (invalid) { error.value = invalid; return; }
    await props.client.saveLog({ id: existing.value?.id, ...draft });
    notice.value = 'logSaved';
  });
}
async function remove() {
  const target = confirming.value;
  confirming.value = null;
  if (!target) return;
  await run(async () => { await props.client.deleteLog(target.id); notice.value = ''; });
}
</script>

<template>
  <div class="log-page">
    <p v-if="error" class="banner error" role="alert">{{ t(errorText(error)) }}</p>
    <p v-if="notice" class="banner" role="status">{{ t(notice) }}</p>
    <article class="card">
      <h2>{{ t('dailyLog') }}</h2>
      <form @submit.prevent="save">
        <label>{{ t('today') }}<input v-model="date" type="date" :max="ymd(new Date())" required></label>
        <button class="log-today" type="button" :disabled="busy" @click="setToday">{{ t('logToday') }}</button>
        <label>{{ t('painLevel') }}<input v-model.number="form.pain_level" type="range" min="0" max="10" step="1"><input v-model.number="form.pain_level" type="number" min="0" max="10" step="1" :aria-label="t('painLevel')"></label>
        <label>{{ t('mood') }}<select v-model="form.mood"><option value="">—</option><option v-for="option in moodOptions" :key="option.value" :value="option.value">{{ t(option.key) }}</option></select></label>
        <div class="log-group" role="group" :aria-label="t('symptoms')">
          <span class="log-label">{{ t('symptoms') }}</span>
          <div class="log-chips">
            <button v-for="option in symptomOptions" :key="option.value" type="button" class="log-chip" :class="{ on: form.symptoms.includes(option.value) }" :aria-pressed="form.symptoms.includes(option.value)" @click="toggleSymptom(option.value)">{{ t(option.key) }}</button>
          </div>
        </div>
        <label>{{ t('periodNote') }}<textarea v-model="form.note" rows="3" maxlength="500"></textarea></label>
        <div class="log-actions">
          <button class="primary" type="submit" :disabled="busy">{{ busy ? t('busy') : t('saveNote') }}</button>
          <button v-if="existing" class="log-danger" type="button" :disabled="busy" @click="confirming = existing">{{ t('delete') }}</button>
        </div>
      </form>
    </article>

    <div v-if="confirming" class="modal-backdrop" @click.self="confirming = null">
      <section class="confirm-card" role="dialog" aria-modal="true">
        <p>{{ t('delete') }} · {{ t('dailyLog') }}</p>
        <div class="confirm-actions">
          <button class="outline pill" type="button" @click="confirming = null">{{ t('cancel') }}</button>
          <button class="primary pill" type="button" :disabled="busy" @click="remove">{{ busy ? t('busy') : t('delete') }}</button>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.log-page { width: 100%; max-width: 100%; min-width: 0; overflow-wrap: anywhere; }
button { min-height: 44px; }
.log-today { margin: 8px 0 0; border: 1px solid var(--accent); background: transparent; color: var(--accent); border-radius: 999px; padding: 10px 16px; font-size: 14px; }
.log-group { margin: 16px 0; }
.log-label { display: block; color: var(--muted); font-size: 13px; margin-bottom: 8px; }
.log-chips { display: flex; flex-wrap: wrap; gap: 8px; }
.log-chip { border: 1px solid var(--line); background: var(--soft); color: var(--ink); border-radius: 999px; padding: 8px 14px; font-size: 13px; }
.log-chip.on { border-color: var(--accent); color: var(--accent); background: transparent; font-weight: 650; }
.log-actions { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 16px; }
.log-actions button { flex: 1 1 120px; min-width: 0; padding: 12px 16px; font-size: 14px; }
.log-actions .primary { background: var(--accent); color: var(--bg); border: 1px solid var(--accent); border-radius: 999px; font-weight: 650; }
.log-danger { border: 1px solid var(--line); background: transparent; color: var(--muted); border-radius: 999px; }
input[type='range'] { padding: 0; min-height: 44px; background: transparent; }
input[type='number'] { min-height: 44px; }
</style>
