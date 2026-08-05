<script setup>
import { ref, onMounted, onUnmounted, computed, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import uPlot from 'uplot'
import 'uplot/dist/uPlot.min.css'
import { getContainer, containerAction, removeContainer, updateContainer, wsURL } from '@/api/client'

const route = useRoute()
const loading = ref(true)
const detail = ref(null)
const logs = ref([])
const logEl = ref(null)
const logAutoScroll = ref(true)
const logShowTS = ref(true)
const chartEl = ref(null)
const memChartEl = ref(null)
let cpuChart = null
let memChart = null
let statsWS = null
let logsWS = null

const container = computed(() => detail.value?.container)

const displayLogs = computed(() =>
  logs.value.map((line) => formatLogLine(line, logShowTS.value)).join('\n'),
)

function formatLogLine(line, showTS) {
  if (showTS) return line
  const m = line.match(/^\d{4}-\d{2}-\d{2}T\S+\s+(.*)$/)
  return m ? m[1] : line
}

function scrollLogsToBottom() {
  nextTick(() => {
    const el = logEl.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

function onLogScroll() {
  const el = logEl.value
  if (!el) return
  const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 24
  logAutoScroll.value = atBottom
}

function toggleAutoScroll() {
  logAutoScroll.value = !logAutoScroll.value
  if (logAutoScroll.value) scrollLogsToBottom()
}

async function load() {
  loading.value = true
  detail.value = await getContainer(route.params.id)
  loading.value = false
  renderCharts(detail.value?.stats?.points ?? [])
}

function renderCharts(points) {
  if (!chartEl.value || !memChartEl.value) return
  const ts = points.map((p) => p.timestamp ? new Date(p.timestamp).getTime() / 1000 : 0)
  const cpu = points.map((p) => p.cpu_pct ?? 0)
  const mem = points.map((p) => p.mem_pct ?? 0)
  const opts = (label, color) => ({
    width: chartEl.value.clientWidth,
    height: 160,
    series: [{}, { label, stroke: color, width: 2 }],
    axes: [{ stroke: '#6b7280' }, { stroke: '#6b7280' }],
    scales: { x: { time: true } },
  })
  if (cpuChart) cpuChart.destroy()
  if (memChart) memChart.destroy()
  cpuChart = new uPlot(opts('CPU %', '#1ebe8a'), [ts, cpu], chartEl.value)
  memChart = new uPlot(opts('Memory %', '#3b82f6'), [ts, mem], memChartEl.value)
}

function connectStreams() {
  const id = route.params.id
  statsWS = new WebSocket(wsURL(`/ws/stats?id=${encodeURIComponent(id)}`))
  statsWS.onmessage = (ev) => {
    const stats = JSON.parse(ev.data)
    renderCharts(stats.points ?? [])
  }
  logsWS = new WebSocket(wsURL(`/ws/logs/${encodeURIComponent(id)}`))
  logsWS.onmessage = (ev) => {
    logs.value.push(ev.data)
    if (logs.value.length > 500) logs.value.shift()
    if (logAutoScroll.value) scrollLogsToBottom()
  }
}

async function act(action) {
  if (action === 'update') {
    await updateContainer(route.params.id)
  } else {
    await containerAction(route.params.id, action)
  }
  await load()
}

async function remove() {
  if (!confirm('Remove this container?')) return
  await removeContainer(route.params.id)
  history.back()
}

onMounted(async () => {
  await load()
  connectStreams()
})

onUnmounted(() => {
  statsWS?.close()
  logsWS?.close()
  cpuChart?.destroy()
  memChart?.destroy()
})
</script>

<template>
  <div v-if="loading" class="text-slate-500">Loading...</div>
  <div v-else-if="container" class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-semibold">{{ container.name }}</h2>
        <p class="text-sm text-slate-500">{{ container.image }}</p>
      </div>
      <div class="flex gap-2">
        <button class="btn-primary" @click="act('start')">Start</button>
        <button class="btn-ghost" @click="act('stop')">Stop</button>
        <button class="btn-ghost" @click="act('restart')">Restart</button>
        <button class="btn-ghost" @click="act('update')">Update</button>
        <button class="btn-ghost text-red-400" @click="remove">Remove</button>
      </div>
    </div>

    <div class="grid gap-4 md:grid-cols-2">
      <div class="card">
        <h3 class="mb-2 text-sm text-slate-500">CPU</h3>
        <div ref="chartEl" />
      </div>
      <div class="card">
        <h3 class="mb-2 text-sm text-slate-500">Memory</h3>
        <div ref="memChartEl" />
      </div>
    </div>

    <div class="card grid gap-2 text-sm md:grid-cols-3">
      <div><span class="text-slate-500">State:</span> {{ container.state }}</div>
      <div><span class="text-slate-500">Health:</span> {{ container.health || '-' }}</div>
      <div><span class="text-slate-500">Uptime:</span> {{ container.uptime || '-' }}</div>
      <div><span class="text-slate-500">Stack:</span> {{ container.compose_project || '-' }}</div>
      <div><span class="text-slate-500">Service:</span> {{ container.compose_service || '-' }}</div>
      <div><span class="text-slate-500">Restarts:</span> {{ container.restart_count }}</div>
    </div>

    <div class="card">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h3 class="text-sm font-medium text-slate-500">Logs</h3>
        <div class="flex gap-2 text-xs">
          <button class="btn-ghost py-1" @click="logShowTS = !logShowTS">
            Timestamps: {{ logShowTS ? 'on' : 'off' }}
          </button>
          <button class="btn-ghost py-1" @click="toggleAutoScroll">
            Autoscroll: {{ logAutoScroll ? 'on' : 'off' }}
          </button>
        </div>
      </div>
      <pre
        ref="logEl"
        class="max-h-96 overflow-auto rounded-lg bg-canvas-dark p-3 text-xs text-slate-300"
        @scroll="onLogScroll"
      >{{ displayLogs || '(waiting for logs...)' }}</pre>
    </div>
  </div>
</template>
