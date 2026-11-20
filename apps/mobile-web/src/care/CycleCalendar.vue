<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { errorCode, type Client, type PeriodRecord } from '../core';
import { inRange, isPeriodStart, monthGrid, monthLabel, periodDays, validatePeriod, ymd } from './cycle';

const props = defineProps<{ client: Client; t: (key: string) => string }>();

// The server sends snake_case codes; text() falls back to server_error for unknown keys.
const ERR: Record<string, string> = { partner_no_share: 'partnerNoShare', cycle_overlap: 'cycleOverlap', care_limit: 'careLimit', conflict: 'conflict', upgrade_required: 'upgrade_required', relationship_changed: 'relationship_changed', not_bound: 'not_bound', invalid_input: 'invalid_input' };
const errorText = (code: string) => ERR[code] || code;

type Flow = 'light' | 'medium' | 'heavy';
type Draft = { id: string; start_date: string; end_date: string; flow_level: Flow; note: string };

const busy = ref(false), error = ref(''), notice = ref('');
const cursor = ref({ year: new Date().getFullYear(), month: new Date().getMonth() + 1 });
const form = ref<Draft | null>(null);
const confirming = ref<PeriodRecord | null>(null);

const locale = (): 'zh' | 'en' => (props.t('dayUnit') === 'days' ? 'en' : 'zh');
const grid = computed(() => monthGrid(cursor.value.year, cursor.value.month));
const label = computed(() => monthLabel(cursor.value.year, cursor.value.month, locale()));
const weekdays = computed(() => locale() === 'en' ? ['M', 'T', 'W', 'T', 'F', 'S', 'S'] : ['一', '二', '三', '四', '五', '六', '日']);
const flowOptions: Array<{ value: Flow; key: string }> = [
  { value: 'light', key: 'flowLight' },
  { value: 'medium', key: 'flowMedium' },
  { value: 'heavy', key: 'flowHeavy' },
];
const periods = computed(() => props.client.care.periods);
const marked = computed(() => periodDays(periods.value));
const logDays = computed(() => new Set(props.client.care.logs.map(log => log.log_date)));

function today() { return ymd(new Date()); }
function shift(delta: number) {
  let year = cursor.value.year, month = cursor.value.month + delta;
  if (month < 1) { month = 12; year--; }
  if (month > 12) { month = 1; year++; }
  return { year, month };
}
const prevLabel = computed(() => { const next = shift(-1); return monthLabel(next.year, next.month, locale()); });
const nextLabel = computed(() => { const next = shift(1); return monthLabel(next.year, next.month, locale()); });
function turn(delta: number) { cursor.value = shift(delta); }
function sameMonth(date: string) { return Number(date.slice(0, 4)) === cursor.value.year && Number(date.slice(5, 7)) === cursor.value.month; }
function dayNumber(date: string) { return Number(date.slice(8, 10)); }

async function run(fn: () => Promise<void>) {
  if (busy.value) return;
  busy.value = true; error.value = ''; notice.value = '';
  try { await fn(); }
  catch (e) { error.value = errorCode(e); }
  finally { busy.value = false; }
}
onMounted(() => {
  if (!props.client.state.couple || !props.client.care.supported) return;
  void run(async () => { await Promise.all([props.client.loadPeriods(), props.client.loadLogs()]); });
});

function open(date: string) {
  const covering = periods.value.find(period => inRange(date, period.start_date, period.end_date));
  notice.value = ''; error.value = '';
  if (covering) form.value = { id: covering.id, start_date: covering.start_date, end_date: covering.end_date || '', flow_level: covering.flow_level, note: covering.note };
  else form.value = { id: '', start_date: date, end_date: '', flow_level: 'medium', note: '' };
}
function close() { form.value = null; }

function overlaps(start: string, end: string, id: string) {
  const stop = end || start;
  return periods.value.some(period => {
    if (period.id === id) return false;
    const periodEnd = period.end_date || period.start_date;
    return start <= periodEnd && period.start_date <= stop;
  });
}
function setFlow(value: Flow) { if (form.value) form.value.flow_level = value; }

async function save() {
  const draft = form.value;
  if (!draft) return;
  await run(async () => {
    const input = { start_date: draft.start_date, end_date: draft.end_date || null, flow_level: draft.flow_level, note: draft.note };
    const invalid = validatePeriod(input, today());
    if (invalid) { error.value = invalid; return; }
    if (overlaps(draft.start_date, draft.end_date, draft.id)) { error.value = 'cycle_overlap'; return; }
    await props.client.savePeriod({ id: draft.id || undefined, ...input });
    form.value = null;
    notice.value = 'saved';
  });
}

const undo = ref<{ label: string; restore: () => Promise<void> } | null>(null);
let undoTimer: ReturnType<typeof setTimeout> | undefined;
// A delete stays recoverable for a moment: undo re-creates the record with a new id.
function armUndo(label: string, restore: () => Promise<void>) {
  undo.value = { label, restore };
  if (undoTimer) clearTimeout(undoTimer);
  undoTimer = setTimeout(() => { undo.value = null; }, 10000);
}
async function revert() {
  const pending = undo.value;
  undo.value = null;
  if (undoTimer) clearTimeout(undoTimer);
  if (pending) await run(pending.restore);
}
async function remove() {
  const target = confirming.value;
  confirming.value = null;
  if (!target) return;
  const snapshot = { start_date: target.start_date, end_date: target.end_date, flow_level: target.flow_level, note: target.note };
  await run(async () => {
    await props.client.deletePeriod(target.id);
    form.value = null;
    armUndo(props.t('cycleCalendar'), async () => { await props.client.savePeriod(snapshot); });
  });
}
function askDelete() { if (form.value && form.value.id) { const target = periods.value.find(period => period.id === form.value?.id); if (target) confirming.value = target; } }
</script>

<template>
  <div class="cal-page">
    <p v-if="error" class="banner error" role="alert">{{ t(errorText(error)) }}</p>
    <p v-if="notice" class="banner" role="status">{{ t(notice) }}</p>
    <p v-if="undo" class="banner undo-bar" role="status">{{ t('deleted') }} · {{ undo.label }} <button type="button" class="undo-action" @click="revert">{{ t('undo') }}</button></p>
    <article class="card cal-card">
      <div class="cal-head">
        <button class="cal-nav" type="button" :aria-label="prevLabel" :disabled="busy" @click="turn(-1)">‹</button>
        <strong class="cal-month" aria-live="polite">{{ label }}</strong>
        <button class="cal-nav" type="button" :aria-label="nextLabel" :disabled="busy" @click="turn(1)">›</button>
      </div>
      <div class="cal-week" aria-hidden="true"><span v-for="(name, index) in weekdays" :key="index">{{ name }}</span></div>
      <div class="cal-grid">
        <button v-for="date in grid" :key="date" type="button" class="cal-day" :class="{ outside: !sameMonth(date), period: marked.has(date), start: isPeriodStart(periods, date), logged: logDays.has(date), today: date === today() }" @click="open(date)">
          <span class="cal-num">{{ dayNumber(date) }}</span>
          <i v-if="logDays.has(date)" class="cal-dot" aria-hidden="true"></i>
        </button>
      </div>
      <p class="subtle cal-legend"><span class="cal-key period"></span>{{ t('cycleCalendar') }} <span class="cal-key dot"></span>{{ t('dailyLog') }}</p>
      <p v-if="!periods.length" class="subtle">{{ t('noCycleData') }}</p>
    </article>

    <article v-if="form" class="card">
      <h2>{{ t(form.id ? 'editPeriod' : 'addPeriod') }}</h2>
      <form @submit.prevent="save">
        <label>{{ t('periodStart') }}<input v-model="form.start_date" type="date" :max="today()" required></label>
        <label>{{ t('periodEnd') }}<input v-model="form.end_date" type="date" :max="today()"></label>
        <div class="cal-flows" role="group">
          <button v-for="option in flowOptions" :key="option.value" type="button" class="cal-flow" :class="{ on: form.flow_level === option.value }" :aria-pressed="form.flow_level === option.value" @click="setFlow(option.value)">{{ t(option.key) }}</button>
        </div>
        <label>{{ t('periodNote') }}<textarea v-model="form.note" rows="3" maxlength="500"></textarea></label>
        <div class="cal-actions">
          <button class="primary" type="submit" :disabled="busy">{{ busy ? t('busy') : t('saveNote') }}</button>
          <button class="outline" type="button" :disabled="busy" @click="close">{{ t('cancel') }}</button>
          <button v-if="form.id" class="cal-danger" type="button" :disabled="busy" @click="askDelete">{{ t('delete') }}</button>
        </div>
      </form>
    </article>

    <div v-if="confirming" class="modal-backdrop" @click.self="confirming = null">
      <section class="confirm-card" role="dialog" aria-modal="true">
        <p>{{ t('deletePeriodConfirm') }}</p>
        <div class="confirm-actions">
          <button class="outline pill" type="button" @click="confirming = null">{{ t('cancel') }}</button>
          <button class="primary pill" type="button" :disabled="busy" @click="remove">{{ busy ? t('busy') : t('delete') }}</button>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.undo-bar{display:flex;align-items:center;gap:10px;flex-wrap:wrap}
.undo-action{min-height:36px;padding:4px 14px;border-radius:999px;background:var(--accent);color:var(--bg);font-size:13px;font-weight:650}

.cal-page { width: 100%; max-width: 100%; min-width: 0; overflow-wrap: anywhere; }
button { min-height: 44px; }
.cal-card { padding-bottom: 12px; }
.cal-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 10px; }
.cal-month { font-size: 16px; min-width: 0; overflow-wrap: anywhere; }
.cal-nav { width: 44px; padding: 0; border-radius: 999px; border: 1px solid var(--line); background: var(--soft); color: var(--accent); font-size: 20px; line-height: 1; }
.cal-week, .cal-grid { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 2px; }
.cal-week span { text-align: center; font-size: 11px; color: var(--muted); padding: 4px 0; }
.cal-day { position: relative; min-height: 44px; min-width: 0; border-radius: 12px; border: 1px solid transparent; background: transparent; color: var(--ink); font-size: 13px; padding: 4px 0; }
.cal-day.outside { color: var(--muted); opacity: .55; }
.cal-day.today { border-color: var(--accent); }
.cal-day.period { background: var(--soft); color: var(--accent); font-weight: 650; }
.cal-day.start::after { content: ''; position: absolute; left: 50%; bottom: 3px; width: 4px; height: 4px; margin-left: -2px; border-radius: 50%; background: var(--accent); }
.cal-num { display: block; line-height: 1.2; }
.cal-dot { position: absolute; right: 6px; top: 6px; width: 5px; height: 5px; border-radius: 50%; background: var(--ink); opacity: .5; }
.cal-legend { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.cal-key { display: inline-block; width: 10px; height: 10px; border-radius: 3px; background: var(--soft); border: 1px solid var(--line); }
.cal-key.dot { width: 5px; height: 5px; border-radius: 50%; background: var(--ink); border: none; opacity: .5; }
.cal-flows { display: flex; gap: 8px; flex-wrap: wrap; margin: 12px 0; }
.cal-flow { flex: 1 1 0; min-width: 0; border-radius: 999px; border: 1px solid var(--line); background: var(--soft); color: var(--ink); font-size: 13px; padding: 8px 10px; }
.cal-flow.on { border-color: var(--accent); color: var(--accent); font-weight: 650; background: transparent; }
.cal-actions { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 14px; }
.cal-actions button { flex: 1 1 96px; min-width: 0; padding: 12px 14px; font-size: 14px; }
.cal-actions .primary { background: var(--accent); color: var(--bg); border: 1px solid var(--accent); border-radius: 999px; font-weight: 650; }
.cal-actions .outline { border: 1px solid var(--accent); color: var(--accent); background: transparent; border-radius: 999px; }
.cal-danger { border: 1px solid var(--line); background: transparent; color: var(--muted); border-radius: 999px; }
</style>
