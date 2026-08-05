<script setup>
import { ref, onMounted } from 'vue'
import { getVolumes, removeVolume } from '@/api/client'

const volumes = ref([])

onMounted(async () => {
  volumes.value = await getVolumes()
})

async function remove(name) {
  if (!confirm(`Remove volume ${name}?`)) return
  await removeVolume(name)
  volumes.value = await getVolumes()
}
</script>

<template>
  <div class="space-y-4">
    <h2 class="text-xl font-semibold">Volumes</h2>
    <div class="card overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="text-slate-500">
          <tr>
            <th class="pb-2">Name</th>
            <th class="pb-2">Driver</th>
            <th class="pb-2">Scope</th>
            <th class="pb-2">Status</th>
            <th class="pb-2"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="v in volumes" :key="v.name" class="border-t border-slate-800">
            <td class="py-2 font-mono text-xs">{{ v.name }}</td>
            <td class="py-2">{{ v.driver }}</td>
            <td class="py-2">{{ v.scope }}</td>
            <td class="py-2">
              <span v-if="v.unused" class="badge badge-warn">unused</span>
            </td>
            <td class="py-2">
              <button class="btn-ghost text-xs text-red-400" @click="remove(v.name)">Remove</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
