<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { getPorts } from '@/api/client'

const ports = ref([])
const router = useRouter()

onMounted(async () => {
  ports.value = await getPorts()
})

function stackLabel(p) {
  if (!p.compose_project) return '-'
  if (p.compose_service) return `${p.compose_project} / ${p.compose_service}`
  return p.compose_project
}

function stateClass(state) {
  if (state === 'running') return 'badge-running'
  if (state === 'exited') return 'badge-stopped'
  return 'badge-warn'
}
</script>

<template>
  <div class="space-y-4">
    <p class="text-sm text-muted">Host ports published by running containers.</p>
    <div class="card overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="text-muted">
          <tr>
            <th class="pb-2">Host</th>
            <th class="pb-2">Container</th>
            <th class="pb-2">Proto</th>
            <th class="pb-2">Container</th>
            <th class="pb-2">Stack / service</th>
            <th class="pb-2">State</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="ports.length === 0">
            <td colspan="6" class="py-6 text-center text-muted">No published ports</td>
          </tr>
          <tr v-for="(p, i) in ports" :key="`${p.container_id}-${p.binding}-${i}`" class="table-row-hover">
            <td class="py-2 font-mono">{{ p.host_port }}</td>
            <td class="py-2 font-mono text-muted">{{ p.container_port }}</td>
            <td class="py-2 uppercase text-muted">{{ p.protocol }}</td>
            <td class="py-2">
              <button class="text-accent hover:underline" @click="router.push(`/containers/${p.container_id}`)">
                {{ p.container_name }}
              </button>
            </td>
            <td class="py-2">
              <button
                v-if="p.compose_project"
                class="text-accent hover:underline font-mono text-xs"
                @click="router.push(`/stacks/${p.compose_project}`)"
              >
                {{ stackLabel(p) }}
              </button>
              <span v-else class="text-muted">-</span>
            </td>
            <td class="py-2">
              <span class="badge" :class="stateClass(p.container_state)">{{ p.container_state }}</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
