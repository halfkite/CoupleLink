<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue';
import { ApiError, errorCode, type Client, type Note } from './core';
const props = defineProps<{ client: Client; t: (key: string) => string }>();
const notes = ref<Note[]>([]), busy = ref(false), error = ref('');
const editing = ref<{ id: string; title: string; body: string; imageID: string } | null>(null);
const confirming = ref<Note | null>(null), viewer = ref('');
const picker = ref<HTMLInputElement>(), jumpPicker = ref<HTMLInputElement>(), page = ref<HTMLElement>();
const jumpValue = ref(new Date().toISOString().slice(0, 10)), current = ref(0);
async function run(fn: () => Promise<void>) { if (busy.value) return; busy.value = true; error.value = ''; try { await fn(); } catch (e) { error.value = errorCode(e); } finally { busy.value = false; } }
async function load() { await run(async () => { notes.value = await props.client.listNotes(); await nextTick(); measure(); }); }
onMounted(() => { if (props.client.state.couple) void load(); });
function create() { editing.value = { id: '', title: '', body: '', imageID: '' }; }
function edit(note: Note) { editing.value = { id: note.id, title: note.title, body: note.body, imageID: note.image_id || '' }; }
function cancel() { editing.value = null; }
function readBase64(file: File) { return new Promise<string>((resolve, reject) => { const reader = new FileReader(); reader.onload = () => { const value = String(reader.result || ''); resolve(value.slice(value.indexOf(',') + 1)); }; reader.onerror = () => reject(new ApiError('image_failed')); reader.readAsDataURL(file); }); }
async function pickPhoto(event: Event) {
  const element = event.target as HTMLInputElement, file = element.files?.[0];
  element.value = '';
  if (!file || !editing.value) return;
  await run(async () => {
    if (file.size > 5_500_000) throw new ApiError('image_too_large');
    editing.value!.imageID = await props.client.uploadImage(await readBase64(file));
  });
}
async function save() {
  const draft = editing.value;
  if (!draft) return;
  await run(async () => {
    await props.client.saveNote({ id: draft.id || undefined, title: draft.title, body: draft.body, imageID: draft.imageID });
    editing.value = null;
    notes.value = await props.client.listNotes();
    await nextTick(); measure();
  });
}
async function remove(note: Note) {
  confirming.value = null;
  if (!note.id) { error.value = 'invalid_note'; return; }
  await run(async () => { await props.client.deleteNote(note.id); notes.value = await props.client.listNotes(); await nextTick(); measure(); });
}
const sorted = computed(() => [...notes.value].sort((a, b) => a.created_at < b.created_at ? 1 : -1));
function dayKey(value: string) { const d = new Date(value); return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0'); }
function timeOf(value: string) { return new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }); }
function fullOf(value: string) {
  const d = new Date(value), now = new Date();
  const same = (a: Date, b: Date) => a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
  const day = same(d, now) ? props.t('today') : same(d, new Date(now.getTime() - 86400000)) ? props.t('yesterday') : (d.getMonth() + 1) + '月' + d.getDate() + '日';
  return day + ' ' + timeOf(value);
}
function isDayStart(index: number) {
  const note = sorted.value[index];
  return !note || index === 0 || dayKey(sorted.value[index - 1].created_at) !== dayKey(note.created_at);
}
const currentLabel = computed(() => sorted.value[current.value] ? fullOf(sorted.value[current.value].created_at) : props.t('noNotes'));
let observer: IntersectionObserver | null = null;
function measure() {
  const root = page.value;
  if (!root) return;
  observer?.disconnect();
  observer = new IntersectionObserver(entries => {
    const visible = entries.filter(entry => entry.isIntersecting);
    if (!visible.length) return;
    visible.sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
    current.value = Number((visible[0].target as HTMLElement).dataset.index || 0);
  }, { rootMargin: '-80px 0px -72% 0px', threshold: 0 });
  root.querySelectorAll<HTMLElement>('.ruler-item').forEach(item => observer!.observe(item));
}
function jumpTo(date: string) {
  if (!date) return;
  const index = sorted.value.findIndex(note => dayKey(note.created_at) <= date);
  const target = index < 0 ? sorted.value.length - 1 : index;
  const node = page.value?.querySelectorAll<HTMLElement>('.ruler-item')[target];
  node?.scrollIntoView({ behavior: 'smooth', block: 'start' });
}
function openJump() { jumpPicker.value?.focus(); try { jumpPicker.value?.showPicker?.(); } catch { /* tapping the field still opens it */ } }
async function toggleFeatured(note: Note) {
  if (!note.id) return;
  await run(async () => {
    await props.client.setNoteFeatured(note.id, !note.featured);
    notes.value = await props.client.listNotes();
    await nextTick(); measure();
  });
}
function preview(note: Note) { const text = note.body.trim(); return text.length > 140 ? text.slice(0, 140) + '…' : text; }
</script>
<template>
  <div ref="page" class="notes-page" >
    <div class="notes-head"><div><h1 class="notes-title">{{ t('time') }}</h1><p class="subtle">{{ t('notesHint') }}</p></div><button class="primary pill" type="button" :disabled="busy" @click="create">＋ {{ t('newNote') }}</button></div>
    <p v-if="error" class="banner error" role="alert">{{ t(error) }}</p>

    <form v-if="editing" class="note-editor" @submit.prevent="save">
      <div class="note-editor-head"><span>{{ editing.id ? t('edit') : t('newNote') }}</span><button type="button" class="icon-close" :aria-label="t('cancel')" @click="cancel">×</button></div>
      <input class="note-input" v-model="editing.title" :placeholder="t('noteTitle')" maxlength="120" />
      <textarea class="note-text" v-model="editing.body" :placeholder="t('noteBody')" rows="4" maxlength="4000"></textarea>
      <div v-if="editing.imageID" class="note-photo-wrap"><img v-if="client.imageURL(editing.imageID)" class="note-photo" :src="client.imageURL(editing.imageID)" :alt="t('photo')" @click="viewer = editing.imageID" /><button type="button" class="icon-close on-photo" :aria-label="t('delete')" @click="editing.imageID = ''">×</button></div>
      <div class="note-editor-actions">
        <button type="button" class="outline pill" @click="picker?.click()">{{ t('addPhoto') }}</button>
        <input ref="picker" class="file-input" type="file" accept="image/*" @change="pickPhoto" />
        <button type="button" class="outline pill" @click="cancel">{{ t('cancel') }}</button>
        <button class="primary pill" :disabled="busy || (!editing.title.trim() && !editing.body.trim() && !editing.imageID)">{{ t('saveNote') }}</button>
      </div>
    </form>

    <p v-if="!notes.length && !editing" class="empty note-empty">{{ t('noNotes') }}</p>

    <template v-if="notes.length">
      <div class="ruler-sticky">
        <label class="ruler-date" @click.prevent="openJump"><i class="ruler-dot"></i><span>{{ currentLabel }}</span><input ref="jumpPicker" v-model="jumpValue" class="ruler-input" type="date" @change="jumpTo(jumpValue)" /></label>
        <span class="ruler-count">{{ sorted.length }}</span>
      </div>
      <div class="ruler-list">
        <article v-for="(note, index) in sorted" :key="note.id" class="ruler-item" :class="{ major: isDayStart(index) }" :data-index="index">
          <div class="ruler-rail"><span class="ruler-time">{{ timeOf(note.created_at) }}</span><i class="ruler-tick"></i></div>
          <div class="tl-card" :class="{ featured: note.featured }">
            <span v-if="note.featured" class="featured-badge"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 4.6l2.3 4.7 5.2.8-3.8 3.6.9 5.1-4.6-2.4-4.6 2.4.9-5.1L4.5 10l5.2-.8z" /></svg>{{ t('featured') }}</span>
            <img v-if="note.image_id && client.imageURL(note.image_id)" class="note-cover" :src="client.imageURL(note.image_id)" :alt="t('photo')" loading="lazy" @click="viewer = note.image_id || ''" />
            <div v-else-if="note.image_id" class="note-cover-placeholder">{{ t('photoLoading') }}</div>
            <div class="note-body-wrap">
              <h3 v-if="note.title">{{ note.title }}</h3>
              <p v-if="note.body" class="note-body">{{ preview(note) }}</p>
              <div class="note-actions">
                <button type="button" class="act" :class="{ on: note.featured }" :aria-label="t('featured')" :aria-pressed="note.featured" @click="toggleFeatured(note)"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 4.6l2.3 4.7 5.2.8-3.8 3.6.9 5.1-4.6-2.4-4.6 2.4.9-5.1L4.5 10l5.2-.8z" /></svg><span v-if="note.featured">{{ t('featured') }}</span></button>
                <button type="button" class="act" :aria-label="t('edit')" @click="edit(note)"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4.8 19.2l.5-3.4L15.6 5.5a1.8 1.8 0 0 1 2.6 0l.3.3a1.8 1.8 0 0 1 0 2.6L8.2 18.7z" /></svg></button>
                <button type="button" class="act danger" :aria-label="t('delete')" @click="confirming = note"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5.5 7.5h13M9.5 7.5V5.8h5v1.7M7.5 7.5l.8 10.4a1.4 1.4 0 0 0 1.4 1.3h4.6a1.4 1.4 0 0 0 1.4-1.3l.8-10.4" /></svg></button>
              </div>
            </div>
          </div>
        </article>
      </div>
    </template>

    <div v-if="viewer" class="viewer" @click="viewer = ''"><img v-if="client.imageURL(viewer)" :src="client.imageURL(viewer)" :alt="t('photo')" /></div>
    <div v-if="confirming" class="modal-backdrop" @click.self="confirming = null">
      <section class="confirm-card" role="dialog" aria-modal="true">
        <p>{{ t('deleteNoteConfirm') }}</p>
        <div class="confirm-actions"><button type="button" class="outline pill" @click="confirming = null">{{ t('cancel') }}</button><button type="button" class="primary pill" :disabled="busy" @click="remove(confirming)">{{ t('delete') }}</button></div>
      </section>
    </div>
  </div>
</template>
