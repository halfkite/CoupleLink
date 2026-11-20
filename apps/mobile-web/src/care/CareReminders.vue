<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { errorCode, type Client } from '../core';

const props = defineProps<{ client: Client; t: (key: string) => string }>();

// The server sends snake_case codes; text() falls back to server_error for unknown keys.
const ERR: Record<string, string> = { partner_no_share: 'partnerNoShare', cycle_overlap: 'cycleOverlap', care_limit: 'careLimit', conflict: 'conflict', upgrade_required: 'upgrade_required', relationship_changed: 'relationship_changed', not_bound: 'not_bound', invalid_input: 'invalid_input' };
const errorText = (code: string) => ERR[code] || code;

const busy = ref(false), error = ref(''), notice = ref('');
const profile = computed(() => props.client.care.profile);
const enabled = computed(() => !!profile.value?.reminder_enabled);
const bound = computed(() => !!props.client.state.couple);

async function run(fn: () => Promise<void>) {
  if (busy.value) return;
  busy.value = true; error.value = ''; notice.value = '';
  try { await fn(); }
  catch (e) { error.value = errorCode(e); }
  finally { busy.value = false; }
}
onMounted(() => {
  if (!bound.value || !props.client.care.supported) return;
  void run(async () => { await props.client.loadCareProfile(); });
});

async function toggle(next: boolean) {
  await run(async () => {
    await props.client.saveCareProfile({ reminder_enabled: next });
    notice.value = 'saved';
  });
}
function onToggle(event: Event) {
  const target = event.target as HTMLInputElement;
  void toggle(target.checked);
}
</script>

<template>
  <div class="rem-page">
    <p v-if="error" class="banner error" role="alert">{{ t(errorText(error)) }}</p>
    <p v-if="notice" class="banner" role="status">{{ t(notice) }}</p>
    <article v-if="!bound" class="card empty">{{ t('notBound') }}</article>
    <article v-else class="card">
      <h2>{{ t('careReminders') }}</h2>
      <label class="rem-toggle">
        <span class="rem-name">{{ t('reminderEnabled') }}</span>
        <input type="checkbox" :checked="enabled" :disabled="busy || !profile" @change="onToggle">
      </label>
      <ul class="rem-list">
        <li>{{ t('careReminderUpcoming') }}</li>
        <li>{{ t('careReminderLog') }}</li>
        <li>{{ t('careReminderTask') }}</li>
      </ul>
      <p class="rem-note">{{ t('privateReminder') }}</p>
      <p class="subtle">{{ t('privacyHint') }}</p>
    </article>
  </div>
</template>

<style scoped>
.rem-page { width: 100%; max-width: 100%; min-width: 0; overflow-wrap: anywhere; }
button { min-height: 44px; }
.rem-toggle { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin: 12px 0; padding: 12px 14px; border-radius: 16px; background: var(--soft); color: var(--ink); font-size: 14px; min-height: 52px; }
.rem-name { min-width: 0; overflow-wrap: anywhere; }
.rem-toggle input[type='checkbox'] { display: inline-block; width: 24px; min-width: 24px; height: 24px; min-height: 24px; margin: 0; padding: 0; border-radius: 6px; accent-color: var(--accent); background: var(--card); }
.rem-list { margin: 8px 0 0; padding-left: 18px; color: var(--ink); font-size: 14px; line-height: 1.7; }
.rem-list li { margin: 2px 0; }
.rem-note { margin: 14px 0 6px; padding: 12px 14px; border-radius: 14px; background: var(--soft); color: var(--ink); font-size: 13px; line-height: 1.5; }
</style>
