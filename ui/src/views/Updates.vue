<script setup>
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { getUpdates, scanUpdates, updateContainer, removeImage } from '@/api/client'
import { promptRemovePreviousImage, promptRemovePreviousImages } from '@/lib/images'
import { useEngineStore } from '@/stores/engine'
import { useUiStore } from '@/stores/ui'
import { errorMessage } from '@/lib/errors'

const router = useRouter()
const store = useEngineStore()
const ui = useUiStore()
const report = ref({ pending: [], failures: [], scanned_at: '' })
const scanning = ref(false)
const updatingAll = ref(false)
const updating = ref({})

function anyUpdating() {
  return updatingAll.value || Object.keys(updating.value).length > 0
}

onMounted(() => {
  refresh()
})

watch(() => store.lastEvent, (ev) => {
  if (ev?.type !== 'updates') return
  if (ev.action === 'auto_update' && ev.message) {
    ui.setInfo(ev.message)
  } else if (ev.action === 'auto_update_failed' && ev.message) {
    ui.setError(ev.message)
  }
  refresh()
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
  ui.clear()
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

async function updateAll() {
  const rows = [...(report.value.pending || [])]
  if (!rows.length || anyUpdating()) return

  updatingAll.value = true
  ui.clear()
  const previousImages = []
  const failed = []
  let ok = 0

  for (const row of rows) {
    const id = row.container_id
    updating.value = { ...updating.value, [id]: true }
    try {
      const result = await updateContainer(id)
      ok += 1
      if (result.previous_image) previousImages.push(result.previous_image)
      await refresh()
    } catch (err) {
      failed.push(row.name || id)
    } finally {
      const next = { ...updating.value }
      delete next[id]
      updating.value = next
    }
  }

  if (previousImages.length && await promptRemovePreviousImages(previousImages)) {
    const seen = new Set()
    for (const img of previousImages) {
      if (!img?.id || seen.has(img.id)) continue
      seen.add(img.id)
      try {
        await removeImage(img.id)
      } catch (err) {
        ui.setError(errorMessage(err, 'Failed to remove previous image'))
        break
      }
    }
    await refresh()
  }

  updatingAll.value = false

  if (failed.length) {
    const detail = failed.join(', ')
    if (ok > 0) {
      ui.setError(`Updated ${ok} of ${rows.length}. Failed: ${detail}`)
    } else {
      ui.setError(`Update failed for: ${detail}`)
    }
  } else if (ok > 0) {
    ui.setInfo(ok === 1 ? 'Updated 1 container.' : `Updated ${ok} containers.`)
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
      <button
        v-if="report.pending?.length"
        class="btn-primary"
        :disabled="scanning || anyUpdating()"
        @click="updateAll"
      >
        {{ updatingAll ? 'Updating all…' : 'Update all' }}
      </button>
      <button class="btn-ghost" :disabled="scanning || anyUpdating()" @click="scanNow">
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
              <span v-if="row.auto_update" class="ml-2 badge badge-running text-xs">auto</span>
            </td>
            <td class="py-2 font-mono text-xs">{{ row.image }}</td>
            <td class="py-2 font-mono text-xs text-muted">{{ shortDigest(row.local_digest) }}</td>
            <td class="py-2 font-mono text-xs">{{ shortDigest(row.remote_digest) }}</td>
            <td class="py-2">
              <button
                class="btn-ghost text-xs"
                :disabled="anyUpdating()"
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
