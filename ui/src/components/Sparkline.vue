<script setup>
defineProps({
  points: { type: Array, default: () => [] },
  field: { type: String, default: 'cpu_pct' },
  width: { type: Number, default: 64 },
  height: { type: Number, default: 20 },
})

function path(points, field, w, h) {
  if (!points.length) return ''
  const vals = points.map((p) => p[field] ?? 0)
  const max = Math.max(...vals, 1)
  const step = w / Math.max(vals.length - 1, 1)
  return vals
    .map((v, i) => {
      const x = i * step
      const y = h - (v / max) * h
      return `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
}
</script>

<template>
  <svg :width="width" :height="height" class="inline-block text-accent">
    <path
      :d="path(points, field, width, height)"
      fill="none"
      stroke="currentColor"
      stroke-width="1.5"
    />
  </svg>
</template>
