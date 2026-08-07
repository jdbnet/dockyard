<script setup>
import { ref, onMounted } from 'vue'
import { getNetworks, removeNetwork } from '@/api/client'

const networks = ref([])

onMounted(async () => {
  networks.value = await getNetworks()
})

async function remove(id) {
  if (!confirm('Remove this network?')) return
  await removeNetwork(id)
  networks.value = await getNetworks()
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
            <th class="pb-2">Containers</th>
            <th class="pb-2"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="n in networks" :key="n.id" class="table-row-hover">
            <td class="py-2">{{ n.name }}</td>
            <td class="py-2">{{ n.driver }}</td>
            <td class="py-2">{{ n.scope }}</td>
            <td class="py-2">{{ n.containers }}</td>
            <td class="py-2">
              <button
                v-if="!['bridge','host','none'].includes(n.name)"
                class="btn-ghost text-xs text-danger"
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
