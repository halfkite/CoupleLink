<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue';
import { errorCode, type Client } from './core';
import { nearLatest } from './memories';
const props = defineProps<{ client: Client; t: (key: string) => string }>();
const list = ref<HTMLElement>(), latest = ref(true), search = ref(false), query = ref(''), busy = ref(false), error = ref(''), target = ref(0), searched = ref(false);
const messages = computed(() => props.client.memories.context ?? props.client.state.messages);
async function run(fn: () => Promise<void>) { if (busy.value) return; busy.value = true; error.value = ''; try { await fn(); } catch (e) { error.value = errorCode(e); } finally { busy.value = false; } }
async function scrollState() { if (list.value) latest.value = nearLatest(list.value.scrollTop, list.value.scrollHeight, list.value.clientHeight); await props.client.setChatVisible(latest.value && !props.client.memories.context && !search.value); }
async function bottom() { props.client.closeHistory(); search.value = false; target.value = 0; await nextTick(); list.value?.scrollTo({ top: list.value.scrollHeight, behavior: 'instant' }); latest.value = true; await scrollState(); }
async function open(seq: number) { await props.client.openMessage(seq); target.value = seq; search.value = false; await nextTick(); list.value?.querySelector(`[data-seq="${seq}"]`)?.scrollIntoView({ block: 'center' }); await scrollState(); }
async function doSearch(more = false) { await props.client.searchMessages(query.value, more); searched.value = true; }
async function send() { await props.client.send(); await bottom(); }
async function draft(e: Event) { try { await props.client.draft((e.target as HTMLTextAreaElement).value); } catch (e) { error.value = errorCode(e); } }
function visibility() { void run(scrollState); }
watch(() => props.client.state.messages.length, async () => { if (latest.value && !props.client.memories.context && !search.value) { await nextTick(); list.value?.scrollTo({ top: list.value.scrollHeight, behavior: 'instant' }); } });
watch(search, () => { void run(scrollState); });
watch(() => props.client.state.couple?.id, () => { search.value = false; query.value = ''; searched.value = false; target.value = 0; });
onMounted(() => { void run(bottom); document.addEventListener('visibilitychange', visibility); });
onUnmounted(() => { document.removeEventListener('visibilitychange', visibility); void props.client.setChatVisible(false); });
</script>
<template>
  <div class="chat-page"><div class="heading chat-heading"><div><h1>{{ client.state.couple?.partner.display_name || t('chat') }}</h1><p>{{ t(client.state.online ? 'online' : 'offline') }}</p></div><button v-if="client.state.couple" class="outline" @click="search = !search">⌕ {{ t('searchHistory') }}</button></div>
  <p v-if="error" class="banner error" role="alert">{{ t(error) }}</p>
  <div v-if="!client.state.couple" class="card empty">{{ t('not_bound') }}</div>
  <template v-else>
    <article v-if="search" class="card search-panel"><form @submit.prevent="run(() => doSearch())"><label>{{ t('searchHistory') }}<input v-model="query" type="search" maxlength="100" :placeholder="t('searchHint')" required></label><button class="primary wide" :disabled="busy || !query.trim()">{{ t('search') }}</button></form><p v-if="searched && !client.memories.results.length" class="empty">{{ t('noResults') }}</p><button v-for="m in client.memories.results" :key="m.id" class="search-result" :disabled="busy" @click="run(() => open(m.seq))"><small>{{ new Date(m.created_at).toLocaleString() }} · {{ m.sender_id === client.state.user?.id ? t('me') : client.state.couple.partner.display_name }}</small><p>{{ m.body }}</p><span>{{ t('viewContext') }} →</span></button><button v-if="client.memories.more" class="outline wide" :disabled="busy" @click="run(() => doSearch(true))">{{ t('loadMore') }}</button></article>
    <div v-else class="chat-viewport"><p v-if="client.memories.context" class="history-hint">{{ t('historyContext') }}</p><div class="chat-list" ref="list" @scroll.passive="scrollState"><p v-if="!messages.length" class="empty">{{ t('emptyChat') }} ♡</p><article v-for="m in messages" :key="m.id" :data-seq="m.seq" class="message" :class="{ own: m.sender_id === client.state.user?.id, highlighted: m.seq === target }"><p>{{ m.body }}</p><small>{{ new Date(m.created_at).toLocaleString([], { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) }} · {{ t(m.status || (m.delivered ? 'delivered' : 'sent')) }}</small></article></div><button v-if="!latest || client.memories.context" class="jump-latest" :aria-label="t('latestMessages')" @click="run(bottom)">↓ {{ t('latestMessages') }}<i v-if="client.state.unread" class="unread"></i></button></div>
    <form v-if="!search" class="composer" @submit.prevent="run(send)"><textarea :value="client.state.draft" @input="draft" :placeholder="t('chatHint')" :aria-label="t('chatHint')" rows="2" maxlength="8000"></textarea><button class="primary" :disabled="busy || !client.state.ready || !client.state.draft.trim()">{{ t('send') }}</button></form>
  </template></div>
</template>
