<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useEngineStore } from '@/stores/engine'
import { containerAction, updateContainer, removeImage } from '@/api/client'
import { promptRemovePreviousImage } from '@/lib/images'
import Sparkline from '@/components/Sparkline.vue'

const store = useEngineStore()
const router = useRouter()
const updating = ref({})

onMounted(() => {
  if (store.composeProjects.length === 0) {
    store.fetch()
  }
})

const projects = computed(() => {
  if (store.flatView) {
    const flat = store.composeProjects
    if (flat.length && flat[0].name !== undefined && flat[0].containers) {
      return flat
    }
    return [{ name: 'All containers', containers: store.composeProjects }]
  }
  return store.composeProjects
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
      alert(err.response?.data?.error || err.message || 'Update failed')
    } finally {
      const next = { ...updating.value }
      delete next[id]
      updating.value = next
    }
    return
  }
  await containerAction(id, action)
  await store.fetch()
}

function fmtStat(v) {
  if (v == null || v === undefined || Number.isNaN(v)) return '-'
  return `${v.toFixed(1)}%`
}
</script>

<template>
  <div class="space-y-4">
    <div class="page-toolbar">
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
              <th class="pb-2 pr-4">Name</th>
              <th class="pb-2 pr-4">Status</th>
              <th class="pb-2 pr-4">CPU</th>
              <th class="pb-2 pr-4">Memory</th>
              <th class="pb-2 pr-4">Uptime</th>
              <th class="pb-2 pr-4">Health</th>
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
              <td class="py-2 pr-4">{{ fmtStat(c.mem_pct) }}</td>
              <td class="py-2 pr-4 text-muted">{{ c.uptime || '-' }}</td>
              <td class="py-2 pr-4">
                <span v-if="c.restart_loop" class="badge badge-error">restart loop</span>
                <span v-else class="text-muted">{{ c.health || '-' }}</span>
              </td>
              <td class="py-2 space-x-1">
                <button class="btn-ghost text-xs" @click="act(c.id, 'start')">Start</button>
                <button class="btn-ghost text-xs" @click="act(c.id, 'stop')">Stop</button>
                <button class="btn-ghost text-xs" @click="act(c.id, 'restart')">Restart</button>
                <button
                  class="btn-ghost text-xs"
                  :disabled="updating[c.id]"
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
