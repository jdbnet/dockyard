<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { getImages, removeImage, pruneUnusedImages } from '@/api/client'
import { fmtSize } from '@/lib/images'
import { useEngineStore } from '@/stores/engine'
import { useUiStore } from '@/stores/ui'
import { errorMessage } from '@/lib/errors'

const images = ref([])
const pruning = ref(false)
const removing = ref('')
const store = useEngineStore()
const ui = useUiStore()

const unusedCount = computed(() => images.value.filter((img) => img.unused).length)

onMounted(async () => {
  await refresh()
})

watch(() => store.lastEvent, (ev) => {
  if (ev?.type === 'image' || ev?.type === 'container') refresh()
})

async function refresh() {
  try {
    images.value = await getImages()
  } catch (err) {
    ui.setError(errorMessage(err, 'Failed to load images'))
  }
}

function tag(img) {
  return img.repo_tags?.length ? img.repo_tags.join(', ') : '<none>'
}

async function remove(id) {
  if (!confirm('Remove this image?')) return
  removing.value = id
  try {
    await removeImage(id)
    await refresh()
  } catch (err) {
    ui.setError(errorMessage(err, 'Remove failed'))
  } finally {
    removing.value = ''
  }
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
    ui.setInfo(msg)
    await refresh()
  } catch (err) {
    ui.setError(errorMessage(err, 'Prune failed'))
  } finally {
    pruning.value = false
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="page-toolbar">
      <button
        class="btn-ghost text-sm text-warn-accent disabled:opacity-40"
        :disabled="unusedCount === 0 || pruning"
        @click="pruneUnused"
      >
        Prune unused ({{ unusedCount }})
      </button>
    </div>
    <div class="card overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="text-muted">
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
          <tr v-for="img in images" :key="img.id" class="table-row-hover">
            <td class="py-2 font-mono text-xs">{{ img.short_id }}</td>
            <td class="py-2">{{ tag(img) }}</td>
            <td class="py-2">{{ fmtSize(img.size) }}</td>
            <td class="py-2">{{ img.containers }}</td>
            <td class="py-2">
              <span v-if="img.unused" class="badge badge-warn">unused</span>
            </td>
            <td class="py-2">
              <button
                class="btn-ghost text-xs text-danger"
                :disabled="!!removing"
                @click="remove(img.id)"
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
