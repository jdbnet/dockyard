<script setup>
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { createAPIKey, listAPIKeys, revokeAPIKey } from '@/api/client'
import { errorMessage } from '@/lib/errors'

const keys = ref([])
const name = ref('')
const scope = ref('read')
const createdKey = ref('')
const message = ref('')
const messageError = ref(false)
const saving = ref(false)

function formatWhen(value) {
  if (!value || String(value).startsWith('0001-')) return 'Never'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Never'
  return date.toLocaleString('en-GB')
}

async function load() {
  keys.value = await listAPIKeys()
}

onMounted(async () => {
  try {
    await load()
  } catch (err) {
    message.value = errorMessage(err, 'Failed to load API keys')
    messageError.value = true
  }
})

async function create() {
  saving.value = true
  message.value = ''
  createdKey.value = ''
  try {
    const result = await createAPIKey({ name: name.value, scope: scope.value })
    createdKey.value = result.key
    name.value = ''
    scope.value = 'read'
    await load()
  } catch (err) {
    message.value = errorMessage(err, 'Failed to create API key')
    messageError.value = true
  } finally {
    saving.value = false
  }
}

async function revoke(id) {
  message.value = ''
  try {
    await revokeAPIKey(id)
    await load()
  } catch (err) {
    message.value = errorMessage(err, 'Failed to revoke API key')
    messageError.value = true
  }
}
</script>

<template>
  <div class="mx-auto max-w-3xl space-y-4">
    <div class="flex flex-wrap gap-2 text-sm">
      <RouterLink to="/settings" class="btn-secondary">General</RouterLink>
      <RouterLink to="/settings/api-keys" class="btn-secondary">API keys</RouterLink>
      <RouterLink to="/docs" class="btn-secondary">API docs</RouterLink>
    </div>

    <section class="card space-y-4">
      <div>
        <h1 class="text-lg font-semibold text-heading">API keys</h1>
        <p class="mt-1 text-sm text-muted">
          Other apps can read stats and containers with a key. The full key is shown once.
        </p>
      </div>

      <form class="grid gap-4 sm:grid-cols-[1fr_10rem_auto] sm:items-end" @submit.prevent="create">
        <div>
          <label class="mb-1 block text-sm text-muted" for="key-name">Name</label>
          <input id="key-name" v-model="name" type="text" class="input-field" required />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted" for="key-scope">Scope</label>
          <select id="key-scope" v-model="scope" class="input-field">
            <option value="read">read</option>
            <option value="write">write</option>
          </select>
        </div>
        <button type="submit" class="btn-primary" :disabled="saving">Create</button>
      </form>

      <div v-if="createdKey" class="rounded-lg border border-default bg-surface-overlay p-3 text-sm">
        <p class="text-muted">Copy this key now. It will not be shown again.</p>
        <p class="mt-2 break-all font-mono">{{ createdKey }}</p>
      </div>
      <p v-if="message" class="text-sm" :class="messageError ? 'text-danger' : 'text-accent'">{{ message }}</p>
    </section>

    <section class="card">
      <p v-if="!keys.length" class="text-sm text-muted">No API keys yet.</p>
      <div v-else class="overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead class="text-muted">
            <tr>
              <th class="pb-2 font-medium">Name</th>
              <th class="pb-2 font-medium">Prefix</th>
              <th class="pb-2 font-medium">Scope</th>
              <th class="pb-2 font-medium">Created</th>
              <th class="pb-2 font-medium">Last used</th>
              <th class="pb-2" />
            </tr>
          </thead>
          <tbody>
            <tr v-for="key in keys" :key="key.id" class="border-t border-default">
              <td class="py-2">{{ key.name }}</td>
              <td class="py-2 font-mono">{{ key.prefix }}</td>
              <td class="py-2">{{ key.scope }}</td>
              <td class="py-2">{{ formatWhen(key.created_at) }}</td>
              <td class="py-2">{{ formatWhen(key.last_used_at) }}</td>
              <td class="py-2 text-right">
                <button type="button" class="btn-ghost" @click="revoke(key.id)">Revoke</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
