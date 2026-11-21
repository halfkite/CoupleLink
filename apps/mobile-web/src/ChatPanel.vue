<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue';
import { ApiError, errorCode, type Client, type Message } from './core';
import { nearLatest } from './memories';
const props = defineProps<{ client: Client; t: (key: string) => string }>();
const root = ref<HTMLElement>(), list = ref<HTMLElement>(), input = ref<HTMLTextAreaElement>(), latest = ref(true), search = ref(false), query = ref(''), busy = ref(false), error = ref(''), target = ref(0), searched = ref(false);
const emoji = ref(false);
const active = ref(''), copied = ref(''), viewer = ref(''), picker = ref<HTMLInputElement>();
const emojis = ['❤️', '😊', '🥰', '😘', '🤗', '😴', '😭', '😡', '👍', '🎉', '🌸', '🌙', '☀️', '🍰', '☕', '🐱', '💋', '💤', '🤔', '🙈', '✨', '💐', '🎁', '👫'];
const messages = computed(() => props.client.memories.context ?? props.client.state.messages);
function own(m: Message) { return m.sender_id === props.client.state.user?.id; }
const RECALL_WINDOW_MS = 180_000;
function canRecall(m: Message) { return own(m) && !m.recalled_at && !m.status && Date.now() - new Date(m.created_at).getTime() < RECALL_WINDOW_MS; }
/** Read state stays visible outside the bubble instead of eating bubble space. */
function readMark(m: Message) {
  if (m.status === 'queued') return props.t('queued');
  if (m.status === 'failed') return props.t('failed');
  if (m.seq > 0 && m.seq <= props.client.state.peerRead) return props.t('read');
  return props.t('unread');
}
function fullTime(m: Message) { return new Date(m.created_at).toLocaleString(); }
async function copy(m: Message) {
  try {
    if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(m.body);
    else throw new Error('unsupported');
  } catch {
    const area = document.createElement('textarea');
    area.value = m.body; area.style.position = 'fixed'; area.style.opacity = '0';
    document.body.appendChild(area); area.select();
    try { document.execCommand('copy'); } finally { area.remove(); }
  }
  copied.value = m.id; setTimeout(() => { if (copied.value === m.id) copied.value = ''; }, 1500);
}
async function recall(m: Message) { await props.client.recall(m.id); active.value = ''; }
async function reedit(m: Message) { await props.client.draft(m.body); active.value = ''; await nextTick(); input.value?.focus(); }
async function run(fn: () => Promise<void>) { if (busy.value) return; busy.value = true; error.value = ''; try { await fn(); } catch (e) { error.value = errorCode(e); } finally { busy.value = false; } }
async function scrollState() { if (list.value) latest.value = nearLatest(list.value.scrollTop, list.value.scrollHeight, list.value.clientHeight); await props.client.setChatVisible(latest.value && !props.client.memories.context && !search.value); }
async function bottom() { props.client.closeHistory(); search.value = false; target.value = 0; await nextTick(); list.value?.scrollTo({ top: list.value.scrollHeight, behavior: 'instant' }); latest.value = true; await scrollState(); }
async function open(seq: number) { await props.client.openMessage(seq); target.value = seq; search.value = false; await nextTick(); list.value?.querySelector(`[data-seq="${seq}"]`)?.scrollIntoView({ block: 'center' }); await scrollState(); }
async function doSearch(more = false) { await props.client.searchMessages(query.value, more); searched.value = true; }
function readBase64(file: File) { return new Promise<string>((resolve, reject) => { const reader = new FileReader(); reader.onload = () => { const value = String(reader.result || ''); resolve(value.slice(value.indexOf(',') + 1)); }; reader.onerror = () => reject(new ApiError('image_failed')); reader.readAsDataURL(file); }); }
async function chooseImage() { picker.value?.click(); }
async function sendImage(event: Event) {
  const element = event.target as HTMLInputElement, file = element.files?.[0];
  element.value = '';
  if (!file) return;
  await run(async () => {
    if (file.size > 5_500_000) throw new ApiError('image_too_large');
    const id = await props.client.uploadImage(await readBase64(file));
    await props.client.queueMessage(props.client.state.draft, id);
    await bottom();
  });
}
async function send() { input.value?.focus(); await props.client.send(); await nextTick(); await bottom(); await nextTick(); input.value?.focus(); setTimeout(() => input.value?.focus(), 80); }
async function draft(e: Event) { try { await props.client.draft((e.target as HTMLTextAreaElement).value); } catch (e) { error.value = errorCode(e); } }
async function addEmoji(value: string) { try { await props.client.draft(props.client.state.draft + value); } catch (e) { error.value = errorCode(e); } emoji.value = false; }
function visibility() { void run(scrollState); }
/** Keep the newest message visible when the soft keyboard opens or closes. */
function keepBottom() { if (latest.value && !props.client.memories.context && !search.value) list.value?.scrollTo({ top: list.value.scrollHeight, behavior: 'instant' }); }
watch(() => props.client.state.messages.length, async () => { if (latest.value && !props.client.memories.context && !search.value) { await nextTick(); list.value?.scrollTo({ top: list.value.scrollHeight, behavior: 'instant' }); } });
watch(search, () => { void run(scrollState); });
watch(() => props.client.state.couple?.id, () => { search.value = false; query.value = ''; searched.value = false; target.value = 0; });
let observer: ResizeObserver | undefined;
/** The page wrapper has no intrinsic height, so give the chat a measured one. */
function fit() {
  const page = root.value, main = page?.closest('main');
  if (!page || !main) return;
  const style = getComputedStyle(main);
  const available = main.clientHeight - parseFloat(style.paddingTop) - parseFloat(style.paddingBottom);
  page.style.height = Math.max(160, Math.round(available)) + 'px';
}
onMounted(() => {
  void run(bottom);
  document.addEventListener('visibilitychange', visibility);
  window.visualViewport?.addEventListener('resize', keepBottom);
  fit();
  const main = root.value?.closest('main');
  if (main && typeof ResizeObserver !== 'undefined') { observer = new ResizeObserver(() => fit()); observer.observe(main); }
  window.addEventListener('resize', fit);
});
onUnmounted(() => { document.removeEventListener('visibilitychange', visibility); window.visualViewport?.removeEventListener('resize', keepBottom); window.removeEventListener('resize', fit); observer?.disconnect(); void props.client.setChatVisible(false); });
</script>
<template>
  <div class="chat-page" ref="root"><div class="heading chat-heading"><div><h1>{{ client.state.couple?.partner.display_name || t('chat') }}<span class="online-dot" :class="{ online: client.state.online }" role="img" :aria-label="t(client.state.online ? 'online' : 'offline')" :title="t(client.state.online ? 'online' : 'offline')"></span></h1><p v-if="!client.state.online">{{ t('offline') }}</p></div><button v-if="client.state.couple" class="outline" @click="search = !search">⌕ {{ t('searchHistory') }}</button></div>
  <p v-if="error" class="banner error" role="alert">{{ t(error) }}</p>
  <div v-if="!client.state.couple" class="card empty">{{ t('not_bound') }}</div>
  <template v-else>
    <article v-if="search" class="card search-panel"><form @submit.prevent="run(() => doSearch())"><label>{{ t('searchHistory') }}<input v-model="query" type="search" maxlength="100" :placeholder="t('searchHint')" required></label><button class="primary wide" :disabled="busy || !query.trim()">{{ t('search') }}</button></form><p v-if="searched && !client.memories.results.length" class="empty">{{ t('noResults') }}</p><button v-for="m in client.memories.results" :key="m.id" class="search-result" :disabled="busy" @click="run(() => open(m.seq))"><small>{{ new Date(m.created_at).toLocaleString() }} · {{ m.sender_id === client.state.user?.id ? t('me') : client.state.couple.partner.display_name }}</small><p>{{ m.body }}</p><span>{{ t('viewContext') }} →</span></button><button v-if="client.memories.more" class="outline wide" :disabled="busy" @click="run(() => doSearch(true))">{{ t('loadMore') }}</button></article>
    <div v-else class="chat-viewport"><p v-if="client.memories.context" class="history-hint">{{ t('historyContext') }}</p><div class="chat-list" ref="list" @scroll.passive="scrollState"><p v-if="!messages.length" class="empty">{{ t('emptyChat') }} ♡</p><article v-for="m in messages" :key="m.id" :data-seq="m.seq" class="message" :class="{ own: own(m), highlighted: m.seq === target, recalled: !!m.recalled_at }" @click="active = active === m.id ? '' : m.id"><span v-if="own(m)" class="read-mark">{{ readMark(m) }}</span><p v-if="m.recalled_at">{{ t('recalled') }}</p><template v-else><img v-if="m.image_id && client.imageURL(m.image_id)" class="msg-image" :src="client.imageURL(m.image_id)" :alt="t('photo')" loading="lazy" @click.stop="viewer = m.image_id || ''" /><span v-else-if="m.image_id" class="msg-image-pending">{{ t('photoLoading') }}</span><p v-if="m.body">{{ m.body }}</p></template><div v-if="active === m.id" class="msg-popover" @click.stop><small>{{ fullTime(m) }}</small><div class="msg-actions"><button type="button" @click.stop="copy(m)">{{ copied === m.id ? t('copied') : t('copy') }}</button><button v-if="canRecall(m)" type="button" @click.stop="run(() => recall(m))">{{ t('recall') }}</button><button v-if="m.recalled_at && own(m)" type="button" @click.stop="reedit(m)">{{ t('reedit') }}</button></div></div></article></div><button v-if="!latest || client.memories.context" class="jump-latest" :aria-label="t('latestMessages')" @click="run(bottom)">↓ {{ t('latestMessages') }}<i v-if="client.state.unread" class="unread"></i></button></div>
    <form v-if="!search" class="composer" @submit.prevent="run(send)">
      <button type="button" class="emoji-toggle" tabindex="-1" :aria-label="t('attach')" @mousedown.prevent @click="chooseImage">＋</button><input ref="picker" class="file-input" type="file" accept="image/*" @change="sendImage" /><button type="button" class="emoji-toggle" tabindex="-1" :aria-label="t('emoji')" @mousedown.prevent @click="emoji = !emoji">☺</button>
      <textarea ref="input" :value="client.state.draft" @input="draft" :placeholder="t('chatHint')" :aria-label="t('chatHint')" rows="2" maxlength="8000"></textarea>
      <button class="primary" type="submit" tabindex="-1" @mousedown.prevent :disabled="busy || !client.state.ready || !client.state.draft.trim()">{{ t('send') }}</button>
      <div v-if="emoji" class="emoji-panel"><button v-for="value in emojis" :key="value" type="button" @click="addEmoji(value)">{{ value }}</button></div>
    </form>
  </template></div>
    <div v-if="viewer" class="viewer" @click="viewer = ''"><img v-if="client.imageURL(viewer)" :src="client.imageURL(viewer)" :alt="t('photo')" /></div>
</template>