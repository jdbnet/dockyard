<script setup>
import { ref, onMounted, computed } from 'vue'
import { getImages, removeImage, pruneUnusedImages } from '@/api/client'

const images = ref([])
const pruning = ref(false)

const unusedCount = computed(() => images.value.filter((img) => img.unused).length)

onMounted(async () => {
  images.value = await getImages()
})

async function refresh() {
  images.value = await getImages()
}

function tag(img) {
  return img.repo_tags?.length ? img.repo_tags.join(', ') : '<none>'
}

function fmtSize(b) {
  if (b < 1024) return `${b} B`
  const units = ['KB', 'MB', 'GB']
  let i = -1
  do { b /= 1024; i++ } while (b >= 1024 && i < units.length - 1)
  return `${b.toFixed(1)} ${units[i]}`
}

async function remove(id) {
  if (!confirm('Remove this image?')) return
  await removeImage(id)
  await refresh()
}

async function pruneUnused() {
  if (unusedCount.value === 0) return
  if (!confirm(`Prune ${unusedCount.value} unused image(s)?`)) return
  pruning.value = true
  try {
    const result = await pruneUnusedImages()
    let msg = `Pruned ${result.deleted} of ${result.attempted} image(s), reclaimed ${fmtSize(result.space_reclaimed)}`
    if (result.errors?.length) {
      msg += `\n\nFailed to remove:\n${result.errors.join('\n')}`
    } else if (result.deleted === 0 && result.attempted > 0) {
      msg += '\n\nNo images were removed.'
    }
    alert(msg)
    await refresh()
  } catch (err) {
    alert(err.response?.data?.error || err.message || 'Prune failed')
  } finally {
    pruning.value = false
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between gap-4">
      <h2 class="text-xl font-semibold">Images</h2>
      <button
        class="btn-ghost text-sm text-amber-400 disabled:opacity-40"
        :disabled="unusedCount === 0 || pruning"
        @click="pruneUnused"
      >
        Prune unused ({{ unusedCount }})
      </button>
    </div>
    <div class="card overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="text-slate-500">
          <tr>
            <th class="pb-2">ID</th>
            <th class="pb-2">Tags</th>
            <th class="pb-2">Size</th>
            <th class="pb-2">Containers</th>
            <th class="pb-2">Status</th>
            <th class="pb-2"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="img in images" :key="img.id" class="border-t border-slate-800">
            <td class="py-2 font-mono text-xs">{{ img.short_id }}</td>
            <td class="py-2">{{ tag(img) }}</td>
            <td class="py-2">{{ fmtSize(img.size) }}</td>
            <td class="py-2">{{ img.containers }}</td>
            <td class="py-2">
              <span v-if="img.unused" class="badge badge-warn">unused</span>
            </td>
            <td class="py-2">
              <button class="btn-ghost text-xs text-red-400" @click="remove(img.id)">Remove</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
