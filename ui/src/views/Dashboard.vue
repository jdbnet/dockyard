<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useEngineStore } from '@/stores/engine'
import { containerAction, updateContainer, removeImage } from '@/api/client'
import { promptRemovePreviousImage, fmtBytes } from '@/lib/images'
import { useUiStore } from '@/stores/ui'
import { errorMessage } from '@/lib/errors'
import Sparkline from '@/components/Sparkline.vue'

const store = useEngineStore()
const ui = useUiStore()
const router = useRouter()
const updating = ref({})
const pending = ref({})
const search = ref('')
const sortKey = ref('name')
const sortDir = ref('asc')

const columns = [
  { key: 'name', label: 'Name' },
  { key: 'status', label: 'Status' },
  { key: 'cpu', label: 'CPU' },
  { key: 'memory', label: 'Memory' },
  { key: 'uptime', label: 'Uptime' },
  { key: 'health', label: 'Health' },
]

onMounted(() => {
  if (store.composeProjects.length === 0) {
    store.fetch()
  }
})

function matchesName(name, q) {
  if (!q) return true
  return (name || '').toLowerCase().includes(q)
}

function cmpStr(a, b) {
  return a.localeCompare(b, undefined, { sensitivity: 'base' })
}

function cmpNum(a, b, dir) {
  const aMissing = a == null || Number.isNaN(a)
  const bMissing = b == null || Number.isNaN(b)
  if (aMissing && bMissing) return 0
  if (aMissing) return 1
  if (bMissing) return -1
  return (a - b) * dir
}

function sortValue(c, key) {
  switch (key) {
    case 'name':
      return c.name || ''
    case 'status':
      return c.state || ''
    case 'cpu':
      return c.cpu_pct
    case 'memory':
      return c.mem_bytes
    case 'uptime':
      return c.uptime || ''
    case 'health':
      return c.restart_loop ? 'restart loop' : (c.health || '')
    default:
      return ''
  }
}

function sortContainers(list) {
  const key = sortKey.value
  const dir = sortDir.value === 'asc' ? 1 : -1
  const numeric = key === 'cpu' || key === 'memory'
  return [...list].sort((a, b) => {
    if (numeric) {
      return cmpNum(sortValue(a, key), sortValue(b, key), dir)
    }
    return cmpStr(String(sortValue(a, key)), String(sortValue(b, key))) * dir
  })
}

function setSort(key) {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortKey.value = key
  sortDir.value = 'asc'
}

const projects = computed(() => {
  const q = search.value.trim().toLowerCase()
  let list
  if (store.flatView) {
    const flat = store.composeProjects
    if (flat.length && flat[0].name !== undefined && flat[0].containers) {
      list = flat
    } else {
      list = [{ name: 'All containers', containers: store.composeProjects }]
    }
  } else {
    list = store.composeProjects
  }

  const filtered = list.map((p) => ({
    ...p,
    containers: (p.containers || []).filter((c) => matchesName(c.name, q)),
  })).filter((p) => !q || p.containers.length > 0)

  if (!store.flatView) {
    return filtered
  }
  return filtered.map((p) => ({
    ...p,
    containers: sortContainers(p.containers),
  }))
})

function statusBadge(c) {
  if (updating.value[c.id]) return 'badge-warn'
  if (c.restart_loop) return 'badge-error'
  if (c.state === 'running') return 'badge-running'
  if (c.state === 'exited') return 'badge-stopped'
  return 'badge-warn'
}

function statusLabel(c) {
  if (updating.value[c.id]) return 'updating…'
  return c.state
}

function isBusy(id) {
  return !!updating.value[id] || !!pending.value[id]
}

async function act(id, action) {
  if (action === 'update') {
    updating.value = { ...updating.value, [id]: true }
    try {
      const result = await updateContainer(id)
      await store.fetch()
      if (result.previous_image && await promptRemovePreviousImage(result.previous_image)) {
        await removeImage(result.previous_image.id)
        await store.fetch()
      }
    } catch (err) {
      ui.setError(errorMessage(err, 'Update failed'))
    } finally {
      const next = { ...updating.value }
      delete next[id]
      updating.value = next
    }
    return
  }
  pending.value = { ...pending.value, [id]: action }
  try {
    await containerAction(id, action)
    await store.fetch()
  } catch (err) {
    ui.setError(errorMessage(err, `${action} failed`))
  } finally {
    const next = { ...pending.value }
    delete next[id]
    pending.value = next
  }
}

function fmtStat(v) {
  if (v == null || v === undefined || Number.isNaN(v)) return '-'
  return `${v.toFixed(1)}%`
}
</script>

<template>
  <div class="space-y-4">
    <div class="page-toolbar">
      <input
        v-model="search"
        type="search"
        class="input-field mr-auto w-full max-w-sm"
        placeholder="Search containers"
      />
      <button class="btn-ghost" @click="store.toggleFlat()">
        {{ store.flatView ? 'Compose view' : 'Flat view' }}
      </button>
    </div>

    <div v-for="project in projects" :key="project.name" class="card">
      <h3 class="mb-3 text-sm font-medium uppercase tracking-wide text-muted">
        {{ project.name }}
        <span class="ml-2 text-xs normal-case">({{ project.containers?.length ?? 0 }})</span>
      </h3>

      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead class="text-muted">
            <tr>
              <th v-for="col in columns" :key="col.key" class="pb-2 pr-4">
                <button
                  v-if="store.flatView"
                  type="button"
                  class="inline-flex items-center gap-1 hover:text-heading"
                  @click="setSort(col.key)"
                >
                  {{ col.label }}
                  <span v-if="sortKey === col.key" class="text-accent">{{ sortDir === 'asc' ? '↑' : '↓' }}</span>
                </button>
                <span v-else>{{ col.label }}</span>
              </th>
              <th class="pb-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="c in project.containers"
              :key="c.id"
              class="table-row-hover"
            >
              <td class="py-2 pr-4">
                <button
                  class="font-medium text-accent hover:underline"
                  @click="router.push(`/containers/${c.short_id || c.id}`)"
                >
                  {{ c.name }}
                </button>
              </td>
              <td class="py-2 pr-4">
                <span class="badge" :class="statusBadge(c)">{{ statusLabel(c) }}</span>
              </td>
              <td class="py-2 pr-4">{{ fmtStat(c.cpu_pct) }}</td>
              <td class="py-2 pr-4">{{ fmtBytes(c.mem_bytes) }}</td>
              <td class="py-2 pr-4 text-muted">{{ c.uptime || '-' }}</td>
              <td class="py-2 pr-4">
                <span v-if="c.restart_loop" class="badge badge-error">restart loop</span>
                <span v-else class="text-muted">{{ c.health || '-' }}</span>
              </td>
              <td class="py-2 space-x-1">
                <button class="btn-ghost text-xs" :disabled="isBusy(c.id)" @click="act(c.id, 'start')">Start</button>
                <button class="btn-ghost text-xs" :disabled="isBusy(c.id)" @click="act(c.id, 'stop')">Stop</button>
                <button class="btn-ghost text-xs" :disabled="isBusy(c.id)" @click="act(c.id, 'restart')">Restart</button>
                <button
                  class="btn-ghost text-xs"
                  :disabled="isBusy(c.id)"
                  @click="act(c.id, 'update')"
                >
                  {{ updating[c.id] ? 'Updating…' : 'Update' }}
                </button>
                <button
                  class="btn-ghost text-xs"
                  :disabled="c.state !== 'running'"
                  @click="router.push(`/containers/${c.short_id || c.id}?shell=1`)"
                >
                  Shell
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <p v-if="!projects.length" class="text-muted">No containers found.</p>
  </div>
</template>
