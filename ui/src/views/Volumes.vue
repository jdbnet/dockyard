<script setup>
import { ref, onMounted, watch } from 'vue'
import { getVolumes, removeVolume } from '@/api/client'
import { useEngineStore } from '@/stores/engine'
import { useUiStore } from '@/stores/ui'
import { errorMessage } from '@/lib/errors'

const volumes = ref([])
const removing = ref('')
const store = useEngineStore()
const ui = useUiStore()

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
          <tr v-for="v in volumes" :key="v.name" class="table-row-hover">
            <td class="py-2 font-mono text-xs">{{ v.name }}</td>
            <td class="py-2">{{ v.driver }}</td>
            <td class="py-2">{{ v.scope }}</td>
            <td class="py-2">
              <span v-if="v.unused" class="badge badge-warn">unused</span>
            </td>
            <td class="py-2">
              <button
                class="btn-ghost text-xs text-danger"
                :disabled="!!removing"
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
