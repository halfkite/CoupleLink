<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { errorCode, type CareTask, type Client } from '../core';
import { ymd } from './cycle';

const props = defineProps<{ client: Client; t: (key: string) => string }>();

// The server sends snake_case codes; text() falls back to server_error for unknown keys.
const ERR: Record<string, string> = { partner_no_share: 'partnerNoShare', cycle_overlap: 'cycleOverlap', care_limit: 'careLimit', conflict: 'conflict', upgrade_required: 'upgrade_required', relationship_changed: 'relationship_changed', not_bound: 'not_bound', invalid_input: 'invalid_input' };
const errorText = (code: string) => ERR[code] || code;

type Draft = { id: string; title: string; note: string; due: string; status: 'open' | 'done' };

const busy = ref(false), error = ref(''), notice = ref('');
const form = ref<Draft | null>(null);
const confirming = ref<CareTask | null>(null);
const tasks = computed(() => [...props.client.care.tasks].sort((left, right) => {
  const rank = (left.status === 'done' ? 1 : 0) - (right.status === 'done' ? 1 : 0);
  if (rank !== 0) return rank;
  return (left.due_at || '~').localeCompare(right.due_at || '~');
}));

async function run(fn: () => Promise<void>) {
  if (busy.value) return;
  busy.value = true; error.value = '';
  try { await fn(); }
  catch (e) { error.value = errorCode(e); }
  finally { busy.value = false; }
}
onMounted(() => {
  if (!props.client.state.couple || !props.client.care.supported) return;
  void run(async () => { await props.client.loadCareTasks(); });
});

function begin() { form.value = { id: '', title: '', note: '', due: '', status: 'open' }; notice.value = ''; error.value = ''; }
function edit(task: CareTask) { form.value = { id: task.id, title: task.title, note: task.note, due: task.due_at ? ymd(new Date(task.due_at)) : '', status: task.status }; notice.value = ''; error.value = ''; }
function cancel() { form.value = null; }
function dueOf(task: CareTask) { return task.due_at ? ymd(new Date(task.due_at)) : ''; }
function creatorOf(task: CareTask) {
  return task.created_by_me ? (props.client.state.user?.display_name || '') : (props.client.state.couple?.partner.display_name || '');
}
async function create() {
  const draft = form.value;
  if (!draft) return;
  await run(async () => {
    const title = draft.title.trim();
    if (!title || [...title].length > 120 || [...draft.note].length > 500) { error.value = 'invalid_input'; return; }
    const due = draft.due ? new Date(draft.due + 'T00:00:00') : null;
    const dueAt = due && Number.isFinite(due.getTime()) ? due.toISOString() : null;
    if (draft.id) await props.client.saveCareTask({ id: draft.id, title, note: draft.note, due_at: dueAt, status: draft.status });
    else await props.client.saveCareTask({ title, note: draft.note, due_at: dueAt });
    form.value = null;
    notice.value = 'saved';
  });
}
async function toggle(task: CareTask) {
  await run(async () => {
    await props.client.saveCareTask({ id: task.id, title: task.title, note: task.note, due_at: task.due_at, status: task.status === 'open' ? 'done' : 'open' });
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
  const snapshot = { title: target.title, note: target.note, due_at: target.due_at, status: target.status };
  await run(async () => {
    await props.client.deleteCareTask(target.id);
    armUndo(props.t('careTaskTitle'), async () => { await props.client.saveCareTask(snapshot); });
  });
}
</script>

<template>
  <div class="task-page">
    <p v-if="error" class="banner error" role="alert">{{ t(errorText(error)) }}</p>
    <p v-if="notice" class="banner" role="status">{{ t(notice) }}</p>
    <p v-if="undo" class="banner undo-bar" role="status">{{ t('deleted') }} · {{ undo.label }} <button type="button" class="undo-action" @click="revert">{{ t('undo') }}</button></p>
    <article class="card">
      <div class="task-head">
        <h2>{{ t('careChecklist') }}</h2>
        <button class="task-new" type="button" :disabled="busy" @click="begin">＋ {{ t('newCareTask') }}</button>
      </div>
      <form v-if="form" class="task-form" @submit.prevent="create">
        <h3 class="task-form-title">{{ t(form.id ? 'edit' : 'newCareTask') }}</h3>
        <label>{{ t('careTaskTitle') }}<input v-model="form.title" maxlength="120" required></label>
        <label>{{ t('careTaskNote') }}<textarea v-model="form.note" rows="3" maxlength="500"></textarea></label>
        <label>{{ t('careTaskDue') }}<input v-model="form.due" type="date"></label>
        <div class="task-actions">
          <button class="primary" type="submit" :disabled="busy">{{ busy ? t('busy') : t('saveNote') }}</button>
          <button class="outline" type="button" :disabled="busy" @click="cancel">{{ t('cancel') }}</button>
        </div>
      </form>
      <p v-if="!tasks.length" class="empty">{{ t('noCareTasks') }}</p>
      <ul v-else class="task-list">
        <li v-for="task in tasks" :key="task.id" class="task-row" :class="{ done: task.status === 'done' }">
          <div class="task-main">
            <p class="task-title">{{ task.title }}</p>
            <p v-if="task.note" class="task-note">{{ task.note }}</p>
            <div class="task-meta">
              <span class="task-badge">{{ t(task.status === 'done' ? 'careTaskDone' : 'careTaskOpen') }}</span>
              <span v-if="creatorOf(task)" class="task-badge">{{ creatorOf(task) }}</span>
              <span v-if="dueOf(task)" class="task-badge">{{ t('careTaskDue') }} {{ dueOf(task) }}</span>
            </div>
          </div>
          <div class="task-buttons">
            <button class="task-toggle" type="button" :disabled="busy" @click="toggle(task)">{{ t(task.status === 'done' ? 'careTaskOpen' : 'careTaskDone') }}</button>
            <button class="task-edit" type="button" :disabled="busy" @click="edit(task)">{{ t('edit') }}</button>
            <button class="task-remove" type="button" :disabled="busy" @click="confirming = task">{{ t('delete') }}</button>
          </div>
        </li>
      </ul>
    </article>

    <div v-if="confirming" class="modal-backdrop" @click.self="confirming = null">
      <section class="confirm-card" role="dialog" aria-modal="true">
        <p>{{ t('delete') }} · {{ t('careTaskTitle') }}</p>
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

.task-page { width: 100%; max-width: 100%; min-width: 0; overflow-wrap: anywhere; }
button { min-height: 44px; }
.task-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; flex-wrap: wrap; }
.task-head h2 { margin: 0; }
.task-new { border: 1px solid var(--accent); background: transparent; color: var(--accent); border-radius: 999px; padding: 10px 14px; font-size: 13px; }
.task-form { margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--line); }
.task-form-title { margin: 0 0 6px; font-size: 15px; color: var(--muted); }
.task-actions { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 14px; }
.task-actions button { flex: 1 1 110px; min-width: 0; padding: 12px 14px; font-size: 14px; }
.task-actions .primary { background: var(--accent); color: var(--bg); border: 1px solid var(--accent); border-radius: 999px; font-weight: 650; }
.task-actions .outline { border: 1px solid var(--accent); color: var(--accent); background: transparent; border-radius: 999px; }
.task-list { list-style: none; margin: 8px 0 0; padding: 0; }
.task-row { display: flex; gap: 10px; align-items: flex-start; justify-content: space-between; padding: 14px 0; border-bottom: 1px solid var(--line); }
.task-row:last-child { border-bottom: none; }
.task-main { min-width: 0; flex: 1 1 auto; }
.task-title { margin: 0; font-size: 15px; font-weight: 600; overflow-wrap: anywhere; }
.task-row.done .task-title { text-decoration: line-through; color: var(--muted); }
.task-note { margin: 4px 0 0; color: var(--muted); font-size: 13px; overflow-wrap: anywhere; }
.task-meta { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 8px; }
.task-badge { font-size: 11px; color: var(--muted); background: var(--soft); border-radius: 999px; padding: 3px 8px; }
.task-buttons { display: flex; flex-direction: column; gap: 6px; flex: 0 0 auto; }
.task-toggle, .task-edit, .task-remove { font-size: 12px; border-radius: 999px; padding: 8px 12px; }
.task-toggle { border: 1px solid var(--accent); background: transparent; color: var(--accent); }
.task-edit { border: 1px solid var(--line); background: transparent; color: var(--ink); }
.task-remove { border: 1px solid var(--line); background: transparent; color: var(--muted); }
@media (max-width: 359px) {
  .task-row { flex-direction: column; }
  .task-buttons { flex-direction: row; width: 100%; }
  .task-buttons button { flex: 1 1 0; }
}
</style>
