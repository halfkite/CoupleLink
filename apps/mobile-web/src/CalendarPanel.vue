<script setup lang="ts">
import { watch, ref } from 'vue';
import { newID, errorCode, type Client, type Anniversary } from './core';
import { anniversaryDays } from './memories';
const props = defineProps<{ client: Client; t: (key: string) => string }>();
const busy = ref(false), error = ref(''), saved = ref(false), date = ref(props.client.state.couple?.start_at || ''), dateVersion = ref(props.client.state.couple?.version || 0);
const blank = () => ({ id: newID(), title: '', date: new Date().toLocaleDateString('en-CA'), recurrence: 'yearly' as 'none' | 'yearly', reminder_time: '09:00', version: 0 });
const edit = ref<ReturnType<typeof blank> | null>(null), reminder = ref(true);
async function run(fn: () => Promise<void>, announce = true) { if (busy.value) return; busy.value = true; error.value = ''; saved.value = false; try { await fn(); saved.value = announce; } catch (e) { error.value = errorCode(e); } finally { busy.value = false; } }
function begin(a?: Anniversary) { edit.value = a ? { ...a } : blank(); reminder.value = !a || !!a.reminder_time; saved.value = false; }
async function reload() { await props.client.snapshot(); await props.client.loadAnniversaries(); date.value = props.client.state.couple?.start_at || ''; dateVersion.value = props.client.state.couple?.version || 0; }
async function saveDate() { await props.client.updateStartDate(date.value, dateVersion.value); dateVersion.value = props.client.state.couple!.version; }
async function save() { if (!edit.value) return; await props.client.saveAnniversary({ ...edit.value, reminder_time: reminder.value ? edit.value.reminder_time : '' }, edit.value.version === 0); edit.value = null; }
async function remove() { if(edit.value && window.confirm(props.t('deleteAnniversaryConfirm'))) { await props.client.deleteAnniversary(edit.value.id, edit.value.version); edit.value = null; } }
function countdown(a: Anniversary) { const d = anniversaryDays(a); return d === 0 ? props.t('today') : `${Math.abs(d)} ${props.t(d > 0 ? 'daysUntil' : 'daysAgo')}`; }
watch(() => JSON.stringify([props.client.state.couple?.id, props.client.memories.supported]), () => { edit.value = null; date.value = ''; if(props.client.state.couple && props.client.memories.supported) void run(reload, false); }, { immediate: true });
</script>
<template>
  <div class="heading"><h1>{{ t('time') }}</h1><p>{{ t('memoriesSubtitle') }}</p></div>
  <p v-if="error" class="banner error" role="alert">{{ t(error) }} <button @click="run(reload, false)">{{ t('refresh') }}</button></p>
  <p v-if="saved" class="banner" role="status">{{ t('saved') }}</p>
  <article v-if="!client.state.couple || !client.memories.supported" class="card empty">{{ t(client.state.couple ? 'upgrade_required' : 'not_bound') }}</article>
  <template v-else>
    <article class="card"><h2>{{ t('date') }}</h2><form @submit.prevent="run(saveDate)"><label>{{ t('date') }}<input type="date" v-model="date" :disabled="busy" required></label><button class="primary wide" :disabled="busy">{{ t('saveChanges') }}</button></form><p class="subtle">{{ t('sharedDateHelp') }}</p></article>
    <article class="card"><h2>{{ t('anniversaries') }}<button class="outline" :disabled="busy" @click="begin()">＋ {{ t('add') }}</button></h2><button class="text-button" :disabled="busy" @click="run(reload, false)">{{ t('refresh') }}</button><p v-if="!client.memories.anniversaries.length" class="empty">{{ t('noAnniversaries') }}</p>
      <div v-for="a in client.memories.anniversaries" :key="a.id" class="anniversary-row"><div><strong>{{ a.title }}</strong><p>{{ a.date }} · {{ t(a.recurrence) }}</p><small>{{ t('reminder') }}: {{ a.reminder_time || t('disabled') }} · {{ a.timezone }}</small></div><div><strong class="countdown">{{ countdown(a) }}</strong><button class="text-button" :disabled="busy" @click="begin(a)">{{ t('edit') }}</button></div></div>
      <p class="subtle">{{ t('reminderHelp') }}</p>
    </article>
    <article v-if="edit" class="card"><h2>{{ t(edit.version ? 'editAnniversary' : 'addAnniversary') }}</h2><form @submit.prevent="run(save)"><label>{{ t('title') }}<input v-model="edit.title" required maxlength="80"></label><label>{{ t('anniversaryDate') }}<input type="date" v-model="edit.date" required></label><label>{{ t('repeat') }}<select v-model="edit.recurrence"><option value="none">{{ t('none') }}</option><option value="yearly">{{ t('yearly') }}</option></select></label><label class="toggle">{{ t('reminder') }}<input type="checkbox" v-model="reminder"></label><label v-if="reminder">{{ t('reminderTime') }}<input type="time" v-model="edit.reminder_time" required></label><button class="primary wide" :disabled="busy">{{ t('saveChanges') }}</button></form><div class="button-grid"><button class="text-button" @click="edit = null">{{ t('cancel') }}</button><button v-if="edit.version" class="text-button" :disabled="busy" @click="run(remove)">{{ t('remove') }}</button></div></article>
  </template>
</template>
