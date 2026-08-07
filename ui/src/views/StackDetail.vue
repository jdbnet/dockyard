<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import ComposeEditor from '@/components/ComposeEditor.vue'
import {
  getStackCompose, saveStackCompose, stackUp, stackDown, stackUpdate,
} from '@/api/client'

const route = useRoute()
const name = computed(() => route.params.name)
const content = ref('')
const managed = ref(true)
const editable = ref(false)
const path = ref('')
const loading = ref(true)
const saved = ref(false)
const saveError = ref('')

async function load() {
  loading.value = true
  saveError.value = ''
  const data = await getStackCompose(name.value)
  content.value = data.content
  managed.value = data.managed
  editable.value = data.editable
  path.value = data.path || ''
  loading.value = false
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
  const fn = { up: stackUp, down: stackDown, update: stackUpdate }[action]
  await fn(name.value)
}
</script>

<template>
  <div v-if="loading" class="text-muted">Loading...</div>
  <div v-else class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p v-if="path" class="text-xs font-mono text-muted">{{ path }}</p>
      <div class="flex gap-2 flex-wrap ml-auto">
        <button class="btn-primary" :disabled="!editable" @click="save">Save</button>
        <button class="btn-ghost" @click="act('up')">Up</button>
        <button class="btn-ghost" @click="act('down')">Down</button>
        <button class="btn-ghost" @click="act('update')">Update images</button>
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
