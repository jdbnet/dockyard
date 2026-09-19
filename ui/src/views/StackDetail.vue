<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Loader2 } from '@lucide/vue'
import ComposeEditor from '@/components/ComposeEditor.vue'
import {
  getStackCompose, saveStackCompose, stackUp, stackDown, stackUpdate, deleteStack,
} from '@/api/client'
import { useUiStore } from '@/stores/ui'
import { errorMessage } from '@/lib/errors'

const route = useRoute()
const router = useRouter()
const ui = useUiStore()
const name = computed(() => route.params.name)
const content = ref('')
const managed = ref(true)
const editable = ref(false)
const path = ref('')
const loading = ref(true)
const saved = ref(false)
const saveError = ref('')
const busy = ref('')

const busyLabels = {
  up: 'Starting…',
  down: 'Stopping…',
  update: 'Updating…',
}

async function load() {
  loading.value = true
  saveError.value = ''
  try {
    const data = await getStackCompose(name.value)
    content.value = data.content
    managed.value = data.managed
    editable.value = data.editable
    path.value = data.path || ''
  } catch (err) {
    ui.setError(errorMessage(err, 'Failed to load stack'))
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(name, load)

async function save() {
  if (!editable.value) return
  saveError.value = ''
  try {
    await saveStackCompose(name.value, content.value)
    saved.value = true
    setTimeout(() => { saved.value = false }, 2000)
  } catch (e) {
    saveError.value = e.response?.data?.error || e.message || 'Save failed'
  }
}

async function act(action) {
  busy.value = action
  try {
    const fn = { up: stackUp, down: stackDown, update: stackUpdate }[action]
    await fn(name.value)
  } catch (err) {
    const fallback = { up: 'Up failed', down: 'Down failed', update: 'Update failed' }[action]
    ui.setError(errorMessage(err, fallback))
  } finally {
    busy.value = ''
  }
}

async function remove() {
  if (!managed.value) return
  if (!confirm(`Delete stack ${name.value}?`)) return
  saveError.value = ''
  try {
    await deleteStack(name.value)
    router.push('/stacks')
  } catch (e) {
    ui.setError(errorMessage(e, 'Delete failed'))
  }
}
</script>

<template>
  <div v-if="loading" class="text-muted">Loading...</div>
  <div v-else class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p v-if="path" class="text-xs font-mono text-muted">{{ path }}</p>
      <div class="flex gap-2 flex-wrap ml-auto">
        <button class="btn-primary" :disabled="!editable || !!busy" @click="save">Save</button>
        <button class="btn-ghost gap-1.5" :disabled="!!busy" @click="act('up')">
          <Loader2 v-if="busy === 'up'" class="h-4 w-4 animate-spin" />
          {{ busy === 'up' ? busyLabels.up : 'Up' }}
        </button>
        <button class="btn-ghost gap-1.5" :disabled="!!busy" @click="act('down')">
          <Loader2 v-if="busy === 'down'" class="h-4 w-4 animate-spin" />
          {{ busy === 'down' ? busyLabels.down : 'Down' }}
        </button>
        <button class="btn-ghost gap-1.5" :disabled="!!busy" @click="act('update')">
          <Loader2 v-if="busy === 'update'" class="h-4 w-4 animate-spin" />
          {{ busy === 'update' ? busyLabels.update : 'Update images' }}
        </button>
        <button v-if="managed" class="btn-ghost text-danger" :disabled="!!busy" @click="remove">Delete</button>
        <span v-if="saved" class="text-accent text-sm self-center">Saved</span>
        <span v-if="saveError" class="text-danger text-sm self-center">{{ saveError }}</span>
      </div>
    </div>
    <p class="text-sm text-muted">
      <span v-if="editable">Edit compose.yaml - YAML errors are highlighted inline.</span>
      <span v-else>Read-only - {{ managed ? 'file is not writable' : 'external stack file is not writable' }}.</span>
    </p>
    <ComposeEditor v-model="content" :read-only="!editable" />
  </div>
</template>
