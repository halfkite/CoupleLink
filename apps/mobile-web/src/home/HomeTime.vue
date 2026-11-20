<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import type { Client, Note } from '../core';

const props = defineProps<{ client: Client; t: (key: string) => string }>();
const emit = defineEmits<{ open: [] }>();

const notes = ref<Note[]>([]), loaded = ref(false);
const sorted = computed(() => [...notes.value].sort((a, b) => a.created_at < b.created_at ? 1 : -1));
const featured = computed(() => sorted.value.find(note => note.featured) || null);
const recent = computed(() => sorted.value.filter(note => note.id !== featured.value?.id).slice(0, 3));
// The featured note leads, then the newest three; the featured one is not repeated.
const shown = computed(() => featured.value ? [featured.value, ...recent.value] : recent.value);
const bound = computed(() => !!props.client.state.couple);
const empty = computed(() => loaded.value && !notes.value.length);

async function load() {
  loaded.value = false;
  if (!props.client.state.couple) { loaded.value = true; return; }
  try { notes.value = await props.client.listNotes(); }
  catch { /* offline or unsupported: the preview stays empty instead of breaking home */ }
  finally { loaded.value = true; }
}

onMounted(() => { void load(); });
// The home section mounts before login restore finishes, so reload when pairing lands.
watch(() => props.client.state.couple?.id, () => { void load(); });

function excerpt(body: string) {
  const value = body.replace(/\s+/g, ' ').trim();
  return [...value].length > 60 ? [...value].slice(0, 60).join('') + '…' : value;
}
function dateOf(value: string) { return new Date(value).toLocaleDateString(); }
function photo(note: Note) { return note.image_id ? props.client.imageURL(note.image_id) : ''; }
</script>

<template>
  <article class="card home-section home-time">
    <div class="home-section-head">
      <h2>{{ t('timeRecords') }}</h2>
      <button type="button" class="text-button" @click="emit('open')">{{ t('viewAll') }} ›</button>
    </div>
    <div v-if="shown.length" class="home-time-list">
      <button v-for="note in shown" :key="note.id" type="button" class="home-time-item" :class="{ featured: note.featured }" @click="emit('open')">
        <img v-if="photo(note)" class="home-time-thumb" :src="photo(note)" :alt="t('photo')" loading="lazy" />
        <span class="home-time-text">
          <b v-if="note.title">{{ note.title }}</b>
          <span v-if="note.body" class="home-time-excerpt">{{ excerpt(note.body) }}</span>
          <small class="home-time-date">{{ dateOf(note.created_at) }}</small>
        </span>
      </button>
    </div>
    <p v-else-if="!bound" class="subtle home-empty">{{ t('notBound') }}</p>
    <p v-else-if="empty" class="subtle home-empty">{{ t('noMemoriesYet') }}</p>
  </article>
</template>

<style scoped>
.home-time-thumb{width:52px;height:52px;min-width:52px;border-radius:12px;object-fit:cover;display:block;max-width:100%}
.home-time-text{display:flex;flex-direction:column;gap:3px;min-width:0;flex:1}
.home-time-text b{font-size:14px;font-weight:650;overflow-wrap:anywhere}
.home-time-excerpt{display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;font-size:12.5px;line-height:1.45;color:var(--muted);overflow-wrap:anywhere;min-width:0}
.home-time-date{font-size:11px;color:var(--muted);font-variant-numeric:tabular-nums}
</style>
