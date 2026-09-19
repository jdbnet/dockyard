<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { getVolumes, removeVolume } from '@/api/client'
import { useEngineStore } from '@/stores/engine'
import { useUiStore } from '@/stores/ui'
import { errorMessage } from '@/lib/errors'

const volumes = ref([])
const search = ref('')
const removing = ref('')
const store = useEngineStore()
const ui = useUiStore()

const filteredVolumes = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return volumes.value
  return volumes.value.filter((v) => (v.name || '').toLowerCase().includes(q))
})

onMounted(async () => {
  await refresh()
})

watch(() => store.lastEvent, (ev) => {
  if (ev?.type === 'volume' || ev?.type === 'container') refresh()
})

async function refresh() {
  try {
    volumes.value = await getVolumes()
  } catch (err) {
    ui.setError(errorMessage(err, 'Failed to load volumes'))
  }
}

async function remove(name) {
  const vol = volumes.value.find((v) => v.name === name)
  if (vol && !vol.unused) return
  if (!confirm(`Remove volume ${name}?`)) return
  removing.value = name
  try {
    await removeVolume(name)
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
        placeholder="Search volumes"
      />
    </div>
    <div class="card overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="text-muted">
          <tr>
            <th class="pb-2">Name</th>
            <th class="pb-2">Driver</th>
            <th class="pb-2">Scope</th>
            <th class="pb-2">Status</th>
            <th class="pb-2"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="v in filteredVolumes" :key="v.name" class="table-row-hover">
            <td class="py-2 font-mono text-xs">{{ v.name }}</td>
            <td class="py-2">{{ v.driver }}</td>
            <td class="py-2">{{ v.scope }}</td>
            <td class="py-2">
              <span v-if="v.unused" class="badge badge-warn">unused</span>
            </td>
            <td class="py-2">
              <button
                class="btn-ghost text-xs text-danger"
                :disabled="!!removing || !v.unused"
                :title="v.unused ? '' : 'Volume is in use'"
                @click="remove(v.name)"
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
