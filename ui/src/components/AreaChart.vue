<script setup>
import { computed } from 'vue'

const props = defineProps({
  series: { type: Array, default: () => [] },
  height: { type: Number, default: 88 },
})

const width = 320

const prepared = computed(() => {
  const list = (props.series || []).filter((s) => s.values?.length)
  const len = Math.max(0, ...list.map((s) => s.values.length))
  if (!len) return []
  const flat = list.flatMap((s) => s.values)
  const peak = Math.max(...flat, 0)
  const max = peak > 0 ? peak * 1.15 : 1
  const step = width / Math.max(len - 1, 1)
  return list.map((s) => {
    const coords = s.values.map((v, i) => {
      const x = i * step
      const y = 4 + (1 - (v / max)) * (props.height - 8)
      return [x, y]
    })
    const line = coords.map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`).join(' ')
    const fill = `${line} L${width},${props.height} L0,${props.height} Z`
    return { line, fill, color: s.color || 'var(--color-accent)', fillOpacity: s.fill ? 0.16 : 0 }
  })
})
</script>

<template>
  <svg
    v-if="prepared.length"
    :viewBox="`0 0 ${width} ${height}`"
    class="w-full"
    :style="{ height: `${height}px` }"
    preserveAspectRatio="none"
    aria-hidden="true"
  >
    <path
      v-for="(s, i) in prepared"
      :key="`fill-${i}`"
      :d="s.fill"
      :fill="s.color"
      :fill-opacity="s.fillOpacity"
    />
    <path
      v-for="(s, i) in prepared"
      :key="`line-${i}`"
      :d="s.line"
      fill="none"
      :stroke="s.color"
      stroke-width="1.75"
      vector-effect="non-scaling-stroke"
    />
  </svg>
  <div v-else class="flex items-center text-xs text-muted" :style="{ height: `${height}px` }">
    Waiting for samples
  </div>
</template>
