<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import ComposeEditor from '@/components/ComposeEditor.vue'
import {
  getStacks, createStack, stackUp, stackDown, stackUpdate, deleteStack,
} from '@/api/client'

const stacks = ref([])
const router = useRouter()
const search = ref('')
const updating = ref({})
const showNew = ref(false)
const newName = ref('')
const createError = ref('')
const creating = ref(false)
const newContent = ref(`services:
  app:
    image: nginx:alpine
    ports:
      - "8080:80"
    restart: unless-stopped
`)

onMounted(async () => {
  stacks.value = await getStacks()
})

async function refresh() {
  stacks.value = await getStacks()
}

const filteredStacks = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return stacks.value
  return stacks.value.filter((s) => (s.name || '').toLowerCase().includes(q))
})

async function act(name, action) {
  if (action === 'delete' && !confirm(`Delete stack ${name}?`)) return
  if (action === 'update') {
    updating.value = { ...updating.value, [name]: true }
    try {
      await stackUpdate(name)
      await refresh()
    } catch (err) {
      alert(err.response?.data?.error || err.message || 'Update failed')
    } finally {
      const next = { ...updating.value }
      delete next[name]
      updating.value = next
    }
    return
  }
  const fn = { up: stackUp, down: stackDown, delete: deleteStack }[action]
  await fn(name)
  await refresh()
}

async function submitNew() {
  const name = newName.value.trim()
  if (!name) {
    createError.value = 'Stack name is required'
    return
  }
  if (/[/\\.]/.test(name)) {
    createError.value = 'Stack name cannot contain /, \\, or .'
    return
  }
  createError.value = ''
  const start = confirm('Start stack after creating?')
  creating.value = true
  try {
    await createStack(name, newContent.value, start)
    showNew.value = false
    newName.value = ''
    await refresh()
  } catch (e) {
    createError.value = e.response?.data?.error || e.message || 'Create failed'
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="page-toolbar">
      <input
        v-model="search"
        type="search"
        class="input-field mr-auto w-full max-w-sm"
        placeholder="Search stacks"
      />
      <button class="btn-primary" @click="showNew = !showNew">{{ showNew ? 'Cancel' : 'New stack' }}</button>
    </div>

    <div v-if="showNew" class="card space-y-3">
      <div>
        <label class="mb-1 block text-sm text-muted">Stack name</label>
        <input
          v-model="newName"
          class="input-field font-mono text-sm w-full max-w-sm"
          placeholder="my-stack"
          @input="createError = ''"
        />
      </div>
      <ComposeEditor v-model="newContent" min-height="16rem" />
      <div class="flex items-center gap-3">
        <button class="btn-primary" :disabled="creating" @click="submitNew">
          {{ creating ? 'Creating…' : 'Create stack' }}
        </button>
        <span v-if="createError" class="text-danger text-sm">{{ createError }}</span>
      </div>
    </div>

    <div v-if="filteredStacks.length" class="card overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="text-muted">
          <tr>
            <th class="pb-2">Name</th>
            <th class="pb-2">Running</th>
            <th class="pb-2">Source</th>
            <th class="pb-2">Path</th>
            <th class="pb-2"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in filteredStacks" :key="s.name" class="table-row-hover">
            <td class="py-2">
              <button class="text-accent hover:underline" @click="router.push(`/stacks/${s.name}`)">
                {{ s.name }}
              </button>
            </td>
            <td class="py-2">{{ s.running_count }}/{{ s.container_count }}</td>
            <td class="py-2">
              {{ s.managed ? 'managed' : 'external' }}
              <span v-if="s.editable" class="text-muted"> · editable</span>
            </td>
            <td class="py-2 font-mono text-xs text-muted">{{ s.path }}</td>
            <td class="py-2 space-x-1">
              <button class="btn-ghost text-xs" @click="act(s.name, 'up')">Up</button>
              <button class="btn-ghost text-xs" @click="act(s.name, 'down')">Down</button>
              <button
                class="btn-ghost text-xs"
                :disabled="updating[s.name]"
                @click="act(s.name, 'update')"
              >
                {{ updating[s.name] ? 'Updating…' : 'Update' }}
              </button>
              <button v-if="s.managed" class="btn-ghost text-xs text-danger" @click="act(s.name, 'delete')">Delete</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <p v-else class="text-muted">No stacks found.</p>
  </div>
</template>
