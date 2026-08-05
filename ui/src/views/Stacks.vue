<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  getStacks, createStack, stackUp, stackDown, stackUpdate, deleteStack,
} from '@/api/client'

const stacks = ref([])
const router = useRouter()
const showNew = ref(false)
const newName = ref('')
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

async function act(name, action) {
  if (action === 'delete' && !confirm(`Delete stack ${name}?`)) return
  const fn = { up: stackUp, down: stackDown, update: stackUpdate, delete: deleteStack }[action]
  await fn(name)
  await refresh()
}

async function submitNew() {
  if (!newName.value.trim()) return
  const start = confirm('Start stack after creating?')
  await createStack(newName.value.trim(), newContent.value, start)
  showNew.value = false
  newName.value = ''
  await refresh()
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h2 class="text-xl font-semibold">Compose stacks</h2>
      <button class="btn-primary" @click="showNew = !showNew">{{ showNew ? 'Cancel' : 'New stack' }}</button>
    </div>

    <div v-if="showNew" class="card space-y-3">
      <input v-model="newName" class="input-field font-mono text-sm w-full max-w-sm" placeholder="stack-name" />
      <textarea v-model="newContent" rows="12" class="input-field font-mono text-sm w-full" />
      <button class="btn-primary" @click="submitNew">Create stack</button>
    </div>

    <div class="card overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="text-slate-500">
          <tr>
            <th class="pb-2">Name</th>
            <th class="pb-2">Running</th>
            <th class="pb-2">Source</th>
            <th class="pb-2">Path</th>
            <th class="pb-2"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in stacks" :key="s.name" class="border-t border-slate-800">
            <td class="py-2">
              <button class="text-accent hover:underline" @click="router.push(`/stacks/${s.name}`)">
                {{ s.name }}
              </button>
            </td>
            <td class="py-2">{{ s.running_count }}/{{ s.container_count }}</td>
            <td class="py-2">{{ s.managed ? 'managed' : 'external' }}</td>
            <td class="py-2 font-mono text-xs text-slate-400">{{ s.path }}</td>
            <td class="py-2 space-x-1">
              <button class="btn-ghost text-xs" @click="act(s.name, 'up')">Up</button>
              <button class="btn-ghost text-xs" @click="act(s.name, 'down')">Down</button>
              <button class="btn-ghost text-xs" @click="act(s.name, 'update')">Update</button>
              <button v-if="s.managed" class="btn-ghost text-xs text-red-400" @click="act(s.name, 'delete')">Delete</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
