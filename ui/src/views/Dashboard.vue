<script setup>
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useEngineStore } from '@/stores/engine'
import { containerAction, updateContainer } from '@/api/client'
import Sparkline from '@/components/Sparkline.vue'

const store = useEngineStore()
const router = useRouter()

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
  if (c.restart_loop) return 'badge-error'
  if (c.state === 'running') return 'badge-running'
  if (c.state === 'exited') return 'badge-stopped'
  return 'badge-warn'
}

async function act(id, action) {
  if (action === 'update') {
    await updateContainer(id)
  } else {
    await containerAction(id, action)
  }
  await store.fetch()
}

function fmtStat(v) {
  if (v == null || v === undefined || Number.isNaN(v)) return '—'
  return `${v.toFixed(1)}%`
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h2 class="text-xl font-semibold">Containers</h2>
      <button class="btn-ghost" @click="store.toggleFlat()">
        {{ store.flatView ? 'Compose view' : 'Flat view' }}
      </button>
    </div>

    <div v-for="project in projects" :key="project.name" class="card">
      <h3 class="mb-3 text-sm font-medium uppercase tracking-wide text-slate-500">
        {{ project.name }}
        <span class="ml-2 text-xs normal-case">({{ project.containers?.length ?? 0 }})</span>
      </h3>

      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead class="text-slate-500">
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
              class="border-t border-slate-800 hover:bg-canvas-inset-dark/50"
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
                <span class="badge" :class="statusBadge(c)">{{ c.state }}</span>
              </td>
              <td class="py-2 pr-4">{{ fmtStat(c.cpu_pct) }}</td>
              <td class="py-2 pr-4">{{ fmtStat(c.mem_pct) }}</td>
              <td class="py-2 pr-4 text-slate-400">{{ c.uptime || '-' }}</td>
              <td class="py-2 pr-4">
                <span v-if="c.restart_loop" class="badge badge-error">restart loop</span>
                <span v-else class="text-slate-400">{{ c.health || '-' }}</span>
              </td>
              <td class="py-2 space-x-1">
                <button class="btn-ghost text-xs" @click="act(c.id, 'start')">Start</button>
                <button class="btn-ghost text-xs" @click="act(c.id, 'stop')">Stop</button>
                <button class="btn-ghost text-xs" @click="act(c.id, 'restart')">Restart</button>
                <button class="btn-ghost text-xs" @click="act(c.id, 'update')">Update</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <p v-if="!projects.length" class="text-slate-500">No containers found.</p>
  </div>
</template>
