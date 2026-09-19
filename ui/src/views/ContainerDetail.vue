<script setup>
import { ref, onMounted, onUnmounted, computed, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import uPlot from 'uplot'
import 'uplot/dist/uPlot.min.css'
import { getContainer, containerAction, removeContainer, updateContainer, removeImage, wsURL } from '@/api/client'
import { promptRemovePreviousImage, fmtBytes } from '@/lib/images'
import { useEngineStore } from '@/stores/engine'
import { useUiStore } from '@/stores/ui'
import { errorMessage } from '@/lib/errors'
import TerminalPanel from '@/components/TerminalPanel.vue'

const route = useRoute()
const router = useRouter()
const store = useEngineStore()
const ui = useUiStore()
const loading = ref(true)
const updating = ref(false)
const pending = ref('')
const bottomPanel = ref(route.query.shell === '1' ? 'shell' : 'logs')
const detail = ref(null)
const logs = ref([])
const logEl = ref(null)
const logAutoScroll = ref(true)
const logShowTS = ref(true)
const chartEl = ref(null)
const memChartEl = ref(null)
const liveStats = ref({ cpu_pct: null, mem_pct: null, mem_bytes: null })
let cpuChart = null
let memChart = null
let statsWS = null
let logsWS = null

const container = computed(() => detail.value?.container)
const containerRunning = computed(() => container.value?.state === 'running')
const busy = computed(() => updating.value || !!pending.value)

const displayLogs = computed(() =>
  logs.value.map((line) => formatLogLine(line, logShowTS.value)).join('\n'),
)

const inspectText = computed(() => {
  const raw = detail.value?.inspect
  if (raw == null) return '(no inspect data)'
  if (typeof raw === 'string') {
    try {
      return JSON.stringify(JSON.parse(raw), null, 2)
    } catch {
      return raw
    }
  }
  return JSON.stringify(raw, null, 2)
})

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

function fmtPct(v) {
  if (v == null || Number.isNaN(v)) return '-'
  return `${v.toFixed(1)}%`
}

function applyStatsPoints(points) {
  if (points.length) {
    liveStats.value = points[points.length - 1]
  }
  renderCharts(points)
}

function eventMatchesContainer(ev) {
  if (!ev || ev.type !== 'container') return false
  const c = container.value
  if (!c) return false
  const res = ev.resource || ''
  return res === c.id || res === c.short_id || (c.id && c.id.startsWith(res)) || (res && c.id?.startsWith(res))
}

async function load({ silent = false } = {}) {
  if (!silent) loading.value = true
  try {
    detail.value = await getContainer(route.params.id)
    await nextTick()
    applyStatsPoints(detail.value?.stats?.points ?? [])
  } catch (err) {
    ui.setError(errorMessage(err, 'Failed to load container'))
  } finally {
    loading.value = false
  }
}

function renderCharts(points) {
  if (!chartEl.value || !memChartEl.value) return
  if (!points.length) {
    if (cpuChart) { cpuChart.destroy(); cpuChart = null }
    if (memChart) { memChart.destroy(); memChart = null }
    return
  }
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
    applyStatsPoints(stats.points ?? [])
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
    updating.value = true
    try {
      const result = await updateContainer(route.params.id)
      await load({ silent: true })
      if (result.previous_image && await promptRemovePreviousImage(result.previous_image)) {
        await removeImage(result.previous_image.id)
        await load({ silent: true })
      }
    } catch (err) {
      ui.setError(errorMessage(err, 'Update failed'))
    } finally {
      updating.value = false
    }
    return
  }
  pending.value = action
  try {
    await containerAction(route.params.id, action)
    await load({ silent: true })
  } catch (err) {
    ui.setError(errorMessage(err, `${action} failed`))
  } finally {
    pending.value = ''
  }
}

async function remove() {
  if (!confirm('Remove this container?')) return
  try {
    await removeContainer(route.params.id)
    history.back()
  } catch (err) {
    ui.setError(errorMessage(err, 'Remove failed'))
  }
}

watch(() => store.lastEvent, (ev) => {
  if (eventMatchesContainer(ev)) {
    load({ silent: true })
  }
})

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
  <div v-if="loading" class="text-muted">Loading...</div>
  <div v-else-if="container" class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-semibold text-heading">{{ container.name }}</h2>
        <p class="text-sm text-muted">{{ container.image }}</p>
      </div>
      <div class="flex gap-2">
        <button
          v-if="container.compose_project"
          class="btn-ghost"
          @click="router.push(`/stacks/${encodeURIComponent(container.compose_project)}`)"
        >
          Stack
        </button>
        <button class="btn-primary" :disabled="busy" @click="act('start')">Start</button>
        <button class="btn-ghost" :disabled="busy" @click="act('stop')">Stop</button>
        <button class="btn-ghost" :disabled="busy" @click="act('restart')">Restart</button>
        <button class="btn-ghost" :disabled="busy" @click="act('update')">
          {{ updating ? 'Updating…' : 'Update' }}
        </button>
        <button class="btn-ghost text-danger" :disabled="busy" @click="remove">Remove</button>
      </div>
    </div>

    <div class="grid gap-4 md:grid-cols-2">
      <div class="card">
        <div class="mb-2 flex items-baseline justify-between">
          <h3 class="text-sm text-muted">CPU</h3>
          <span class="text-lg font-semibold text-accent">{{ fmtPct(liveStats.cpu_pct ?? container.cpu_pct) }}</span>
        </div>
        <div ref="chartEl" class="min-h-[160px]" />
        <p v-if="!liveStats.cpu_pct && !container.cpu_pct" class="mt-2 text-xs text-muted">Collecting metrics…</p>
      </div>
      <div class="card">
        <div class="mb-2 flex items-baseline justify-between">
          <h3 class="text-sm text-muted">Memory</h3>
          <span class="text-lg font-semibold text-stat-secondary">{{ fmtBytes(liveStats.mem_bytes ?? container.mem_bytes) }}</span>
        </div>
        <div ref="memChartEl" class="min-h-[160px]" />
        <p v-if="containerRunning && !liveStats.mem_bytes && !container.mem_bytes" class="mt-2 text-xs text-muted">Collecting metrics…</p>
      </div>
    </div>

    <div class="card grid gap-2 text-sm md:grid-cols-3">
      <div><span class="text-muted">State:</span> {{ updating ? 'updating…' : container.state }}</div>
      <div><span class="text-muted">Health:</span> {{ container.health || '-' }}</div>
      <div><span class="text-muted">Uptime:</span> {{ container.uptime || '-' }}</div>
      <div>
        <span class="text-muted">Stack:</span>
        <button
          v-if="container.compose_project"
          class="ml-1 text-accent hover:underline"
          @click="router.push(`/stacks/${encodeURIComponent(container.compose_project)}`)"
        >
          {{ container.compose_project }}
        </button>
        <span v-else> -</span>
      </div>
      <div><span class="text-muted">Service:</span> {{ container.compose_service || '-' }}</div>
      <div><span class="text-muted">Restarts:</span> {{ container.restart_count }}</div>
    </div>

    <div class="card">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <div class="flex gap-2 text-xs">
          <button
            class="btn-ghost py-1"
            :class="{ 'text-accent': bottomPanel === 'logs' }"
            @click="bottomPanel = 'logs'"
          >
            Logs
          </button>
          <button
            class="btn-ghost py-1"
            :class="{ 'text-accent': bottomPanel === 'shell' }"
            :disabled="!containerRunning"
            @click="bottomPanel = 'shell'"
          >
            Shell
          </button>
          <button
            class="btn-ghost py-1"
            :class="{ 'text-accent': bottomPanel === 'inspect' }"
            @click="bottomPanel = 'inspect'"
          >
            Inspect
          </button>
        </div>
        <div v-if="bottomPanel === 'logs'" class="flex gap-2 text-xs">
          <button class="btn-ghost py-1" @click="logShowTS = !logShowTS">
            Timestamps: {{ logShowTS ? 'on' : 'off' }}
          </button>
          <button class="btn-ghost py-1" @click="toggleAutoScroll">
            Autoscroll: {{ logAutoScroll ? 'on' : 'off' }}
          </button>
        </div>
      </div>
      <pre
        v-if="bottomPanel === 'logs'"
        ref="logEl"
        class="log-panel"
        @scroll="onLogScroll"
      >{{ displayLogs || '(waiting for logs...)' }}</pre>
      <pre
        v-else-if="bottomPanel === 'inspect'"
        class="log-panel"
      >{{ inspectText }}</pre>
      <TerminalPanel
        v-else
        :container-id="route.params.id"
        :active="bottomPanel === 'shell'"
        :running="containerRunning"
      />
    </div>
  </div>
  <p v-else class="text-muted">Container not found.</p>
</template>
