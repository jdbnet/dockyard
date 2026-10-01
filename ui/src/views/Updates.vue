<script setup>
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { getUpdates, scanUpdates, updateContainer, removeImage } from '@/api/client'
import { promptRemovePreviousImage } from '@/lib/images'
import { useEngineStore } from '@/stores/engine'
import { useUiStore } from '@/stores/ui'
import { errorMessage } from '@/lib/errors'

const router = useRouter()
const store = useEngineStore()
const ui = useUiStore()
const report = ref({ pending: [], failures: [], scanned_at: '' })
const scanning = ref(false)
const updating = ref({})

onMounted(() => {
  refresh()
})

watch(() => store.lastEvent, (ev) => {
  if (ev?.type === 'updates') refresh()
})

async function refresh() {
  try {
    report.value = await getUpdates()
  } catch (err) {
    ui.setError(errorMessage(err, 'Failed to load updates'))
  }
}

async function scanNow() {
  scanning.value = true
  try {
    report.value = await scanUpdates()
  } catch (err) {
    ui.setError(errorMessage(err, 'Scan failed'))
  } finally {
    scanning.value = false
  }
}

async function update(row) {
  const id = row.container_id
  updating.value = { ...updating.value, [id]: true }
  try {
    const result = await updateContainer(id)
    if (result.previous_image && await promptRemovePreviousImage(result.previous_image)) {
      await removeImage(result.previous_image.id)
    }
    await refresh()
  } catch (err) {
    ui.setError(errorMessage(err, 'Update failed'))
  } finally {
    const next = { ...updating.value }
    delete next[id]
    updating.value = next
  }
}

function shortDigest(value) {
  if (!value) return '-'
  return value.replace(/^sha256:/, '').slice(0, 12)
}

function scannedLabel(value) {
  if (!value || value.startsWith('0001-')) return 'Not scanned yet'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Not scanned yet'
  return date.toLocaleString()
}
</script>

<template>
  <div class="space-y-4">
    <div class="page-toolbar">
      <p class="mr-auto text-sm text-muted">Last scan: {{ scannedLabel(report.scanned_at) }}</p>
      <button class="btn-ghost" :disabled="scanning" @click="scanNow">
        {{ scanning ? 'Scanning…' : 'Scan now' }}
      </button>
    </div>

    <div class="card overflow-x-auto">
      <table v-if="report.pending?.length" class="w-full text-left text-sm">
        <thead class="text-muted">
          <tr>
            <th class="pb-2">Container</th>
            <th class="pb-2">Image</th>
            <th class="pb-2">Current</th>
            <th class="pb-2">Available</th>
            <th class="pb-2"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in report.pending" :key="row.container_id" class="table-row-hover">
            <td class="py-2">
              <button
                class="text-accent hover:underline"
                @click="router.push(`/containers/${row.short_id || row.container_id}`)"
              >
                {{ row.name }}
              </button>
            </td>
            <td class="py-2 font-mono text-xs">{{ row.image }}</td>
            <td class="py-2 font-mono text-xs text-muted">{{ shortDigest(row.local_digest) }}</td>
            <td class="py-2 font-mono text-xs">{{ shortDigest(row.remote_digest) }}</td>
            <td class="py-2">
              <button
                class="btn-ghost text-xs"
                :disabled="!!updating[row.container_id]"
                @click="update(row)"
              >
                {{ updating[row.container_id] ? 'Updating…' : 'Update' }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-else class="py-6 text-center text-muted">No pending updates available.</p>
    </div>

    <div v-if="report.failures?.length" class="card overflow-x-auto">
      <h3 class="mb-3 text-sm font-medium text-muted">Could not check</h3>
      <table class="w-full text-left text-sm">
        <thead class="text-muted">
          <tr>
            <th class="pb-2">Container</th>
            <th class="pb-2">Image</th>
            <th class="pb-2">Error</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in report.failures" :key="`${row.container_id}-fail`" class="table-row-hover">
            <td class="py-2">{{ row.name }}</td>
            <td class="py-2 font-mono text-xs">{{ row.image }}</td>
            <td class="py-2 text-muted">{{ row.error }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
