<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { getNetworks, removeNetwork } from '@/api/client'
import { useEngineStore } from '@/stores/engine'
import { useUiStore } from '@/stores/ui'
import { errorMessage } from '@/lib/errors'

const networks = ref([])
const search = ref('')
const removing = ref('')
const store = useEngineStore()
const ui = useUiStore()

const filteredNetworks = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return networks.value
  return networks.value.filter((n) => (n.name || '').toLowerCase().includes(q))
})

onMounted(async () => {
  await refresh()
})

watch(() => store.lastEvent, (ev) => {
  if (ev?.type === 'network' || ev?.type === 'container') refresh()
})

async function refresh() {
  try {
    networks.value = await getNetworks()
  } catch (err) {
    ui.setError(errorMessage(err, 'Failed to load networks'))
  }
}

async function remove(id) {
  const net = networks.value.find((n) => n.id === id)
  if (net && net.containers > 0) return
  if (!confirm('Remove this network?')) return
  removing.value = id
  try {
    await removeNetwork(id)
    await refresh()
  } catch (err) {
    ui.setError(errorMessage(err, 'Remove failed'))
  } finally {
    removing.value = ''
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="page-toolbar">
      <input
        v-model="search"
        type="search"
        class="input-field mr-auto w-full max-w-sm"
        placeholder="Search networks"
      />
    </div>
    <div class="card overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="text-muted">
          <tr>
            <th class="pb-2">Name</th>
            <th class="pb-2">Driver</th>
            <th class="pb-2">Scope</th>
            <th class="pb-2">Containers</th>
            <th class="pb-2"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="n in filteredNetworks" :key="n.id" class="table-row-hover">
            <td class="py-2">{{ n.name }}</td>
            <td class="py-2">{{ n.driver }}</td>
            <td class="py-2">{{ n.scope }}</td>
            <td class="py-2">{{ n.containers }}</td>
            <td class="py-2">
              <button
                v-if="!['bridge','host','none'].includes(n.name)"
                class="btn-ghost text-xs text-danger"
                :disabled="!!removing || n.containers > 0"
                :title="n.containers > 0 ? 'Network is in use' : ''"
                @click="remove(n.id)"
              >
                Remove
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
