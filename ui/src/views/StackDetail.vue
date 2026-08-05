<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import {
  getStackCompose, saveStackCompose, stackUp, stackDown, stackUpdate,
} from '@/api/client'

const route = useRoute()
const name = computed(() => route.params.name)
const content = ref('')
const managed = ref(true)
const loading = ref(true)
const saved = ref(false)

onMounted(async () => {
  loading.value = true
  const data = await getStackCompose(name.value)
  content.value = data.content
  loading.value = false
})

async function save() {
  await saveStackCompose(name.value, content.value)
  saved.value = true
  setTimeout(() => { saved.value = false }, 2000)
}

async function act(action) {
  const fn = { up: stackUp, down: stackDown, update: stackUpdate }[action]
  await fn(name.value)
}
</script>

<template>
  <div v-if="loading" class="text-slate-500">Loading...</div>
  <div v-else class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-2xl font-semibold">{{ name }}</h2>
      <div class="flex gap-2">
        <button class="btn-primary" @click="save">Save</button>
        <button class="btn-ghost" @click="act('up')">Up</button>
        <button class="btn-ghost" @click="act('down')">Down</button>
        <button class="btn-ghost" @click="act('update')">Update images</button>
        <span v-if="saved" class="text-accent text-sm self-center">Saved</span>
      </div>
    </div>
    <p class="text-sm text-slate-500">Edit compose.yaml - external stacks may be read-only on save.</p>
    <textarea v-model="content" rows="24" class="input-field font-mono text-sm w-full" />
  </div>
</template>
