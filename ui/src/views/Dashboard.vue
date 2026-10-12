<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  Activity, AlertTriangle, ArrowDown, ArrowUp, Box, Cpu, HardDrive, HeartPulse,
  Layers, MemoryStick, Network, Plug, RefreshCw, Timer,
} from '@lucide/vue'
import {
  getContainers, getImages, getLoggingStatus, getNetworks, getPorts, getUpdates, getVolumes,
} from '@/api/client'
import { fmtSize } from '@/lib/images'
import { useEngineStore } from '@/stores/engine'
import { useUiStore } from '@/stores/ui'
import { errorMessage } from '@/lib/errors'
import AreaChart from '@/components/AreaChart.vue'
import Sparkline from '@/components/Sparkline.vue'

const router = useRouter()
const store = useEngineStore()
const ui = useUiStore()

const projects = ref([])
const images = ref([])
const volumes = ref([])
const networks = ref([])
const ports = ref([])
const updates = ref({ pending: [], failures: [] })
const logging = ref(null)

const containers = computed(() => projects.value.flatMap((p) => p.containers || []))
const running = computed(() => containers.value.filter((c) => c.state === 'running'))

function num(v) {
  const n = Number(v)
  return Number.isFinite(n) ? n : 0
}

function sum(list, field) {
  return list.reduce((total, item) => total + num(item[field]), 0)
}

function pointMs(point) {
  const ms = Date.parse(point?.timestamp || '')
  return Number.isNaN(ms) ? null : ms
}

function rate(points, field) {
  if (!points || points.length < 2) return null
  const prev = points[points.length - 2]
  const last = points[points.length - 1]
  const dt = (pointMs(last) - pointMs(prev)) / 1000
  if (!dt || dt <= 0) return null
  const delta = num(last[field]) - num(prev[field])
  if (delta < 0) return null
  return delta / dt
}

function aligned(list, pick) {
  const series = list.map((c) => c.sparkline || []).filter((s) => s.length)
  const len = Math.max(0, ...series.map((s) => s.length))
  if (!len) return []
  const out = []
  for (let i = 0; i < len; i += 1) {
    let value = 0
    for (const spark of series) {
      const point = spark[i - (len - spark.length)]
      if (point) value += pick(point, spark[i - (len - spark.length) - 1])
    }
    out.push(value)
  }
  return out
}

const cpuNow = computed(() => sum(running.value, 'cpu_pct'))
const memNow = computed(() => sum(running.value, 'mem_bytes'))
const rxNow = computed(() => running.value.reduce((total, c) => total + (rate(c.sparkline, 'net_rx') || 0), 0))
const txNow = computed(() => running.value.reduce((total, c) => total + (rate(c.sparkline, 'net_tx') || 0), 0))

const cpuHistory = computed(() => aligned(running.value, (point) => num(point.cpu_pct)))
const memHistory = computed(() => aligned(running.value, (point) => num(point.mem_bytes)))
const netHistory = computed(() => ({
  rx: aligned(running.value, (point, prev) => {
    if (!prev) return 0
    const dt = (pointMs(point) - pointMs(prev)) / 1000
    if (!dt || dt <= 0) return 0
    const delta = num(point.net_rx) - num(prev.net_rx)
    return delta > 0 ? delta / dt : 0
  }),
  tx: aligned(running.value, (point, prev) => {
    if (!prev) return 0
    const dt = (pointMs(point) - pointMs(prev)) / 1000
    if (!dt || dt <= 0) return 0
    const delta = num(point.net_tx) - num(prev.net_tx)
    return delta > 0 ? delta / dt : 0
  }),
}))

const stateCounts = computed(() => {
  const counts = {}
  for (const c of containers.value) {
    const state = c.state || 'unknown'
    counts[state] = (counts[state] || 0) + 1
  }
  return Object.entries(counts).sort((a, b) => b[1] - a[1])
})

const healthCounts = computed(() => {
  const counts = { healthy: 0, unhealthy: 0, starting: 0, none: 0 }
  for (const c of running.value) {
    const health = (c.health || '').toLowerCase()
    if (health === 'healthy' || health === 'unhealthy' || health === 'starting') counts[health] += 1
    else counts.none += 1
  }
  return counts
})

const restartLoops = computed(() => containers.value.filter((c) => c.restart_loop))
const unhealthy = computed(() => running.value.filter((c) => (c.health || '').toLowerCase() === 'unhealthy'))
const flagged = computed(() => {
  const seen = new Set()
  const out = []
  for (const c of [...restartLoops.value, ...unhealthy.value]) {
    if (seen.has(c.id)) continue
    seen.add(c.id)
    out.push(c)
  }
  return out
})
const attention = computed(() => flagged.value.length + (updates.value.pending?.length || 0))

function topContainers(field) {
  return [...running.value].sort((a, b) => num(b[field]) - num(a[field])).slice(0, 5)
}

const topCpu = computed(() => topContainers('cpu_pct'))
const topMem = computed(() => topContainers('mem_bytes'))

const talkers = computed(() => {
  return [...running.value]
    .map((c) => ({
      ...c,
      rx: rate(c.sparkline, 'net_rx') || 0,
      tx: rate(c.sparkline, 'net_tx') || 0,
    }))
    .sort((a, b) => (b.rx + b.tx) - (a.rx + a.tx))
    .slice(0, 5)
})

const projectRows = computed(() => {
  return projects.value
    .map((p) => {
      const list = p.containers || []
      const live = list.filter((c) => c.state === 'running').length
      return {
        name: p.name === '(standalone)' ? 'Standalone' : (p.name || 'Unnamed'),
        total: list.length,
        running: live,
        cpu: sum(list.filter((c) => c.state === 'running'), 'cpu_pct'),
        mem: sum(list.filter((c) => c.state === 'running'), 'mem_bytes'),
      }
    })
    .filter((p) => p.total > 0)
    .sort((a, b) => b.cpu - a.cpu || b.total - a.total)
})

const imageBytes = computed(() => images.value.reduce((total, img) => total + num(img.size), 0))
const unusedImages = computed(() => images.value.filter((img) => img.unused))
const unusedImageBytes = computed(() => unusedImages.value.reduce((total, img) => total + num(img.size), 0))
const unusedVolumes = computed(() => volumes.value.filter((v) => v.unused).length)
const networkContainers = computed(() => networks.value.reduce((total, n) => total + num(n.containers), 0))

const longest = computed(() => {
  return [...running.value]
    .filter((c) => c.started_at && !String(c.started_at).startsWith('0001'))
    .sort((a, b) => Date.parse(a.started_at) - Date.parse(b.started_at))
    .slice(0, 5)
})

const restarts = computed(() => {
  return [...containers.value]
    .filter((c) => num(c.restart_count) > 0)
    .sort((a, b) => num(b.restart_count) - num(a.restart_count))
    .slice(0, 5)
})

const nearLimit = computed(() => {
  return [...running.value]
    .filter((c) => num(c.mem_pct) > 0)
    .sort((a, b) => num(b.mem_pct) - num(a.mem_pct))
    .slice(0, 5)
})

function fmtPct(v) {
  if (v == null || Number.isNaN(Number(v))) return '-'
  const n = Number(v)
  return n >= 100 ? `${n.toFixed(0)}%` : `${n.toFixed(1)}%`
}

function fmtRate(bps) {
  if (bps == null || Number.isNaN(bps)) return '-'
  if (bps < 1024) return `${bps.toFixed(0)} B/s`
  const units = ['KB/s', 'MB/s', 'GB/s']
  let size = bps
  let i = -1
  do { size /= 1024; i += 1 } while (size >= 1024 && i < units.length - 1)
  return `${size.toFixed(size >= 10 ? 0 : 1)} ${units[i]}`
}

function share(value, total) {
  if (!value || !total) return '0%'
  return `${Math.min(100, Math.max(2, (value / total) * 100))}%`
}

function stateColor(state) {
  if (state === 'running') return 'bg-accent'
  if (state === 'restarting' || state === 'paused') return 'bg-amber-400'
  if (state === 'dead') return 'bg-red-500'
  return 'bg-neutral-500'
}

function openContainer(c) {
  router.push(`/containers/${c.short_id || c.id}`)
}

async function loadContainers() {
  projects.value = await getContainers(false)
}

async function loadInventory() {
  const [imgs, vols, nets, published, pending, ship] = await Promise.all([
    getImages(),
    getVolumes(),
    getNetworks(),
    getPorts(),
    getUpdates().catch(() => ({ pending: [], failures: [] })),
    getLoggingStatus().catch(() => null),
  ])
  images.value = imgs || []
  volumes.value = vols || []
  networks.value = nets || []
  ports.value = published || []
  updates.value = pending || { pending: [], failures: [] }
  logging.value = ship
}

async function refreshAll() {
  try {
    await Promise.all([loadContainers(), loadInventory()])
  } catch (err) {
    ui.setError(errorMessage(err, 'Failed to load dashboard'))
  }
}

onMounted(refreshAll)

watch(() => store.lastEvent, (ev) => {
  loadContainers().catch((err) => ui.setError(errorMessage(err, 'Failed to load dashboard')))
  if (ev?.type && ev.type !== 'stats') {
    loadInventory().catch(() => {})
  }
})
</script>

<template>
  <div class="space-y-4">
    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <div class="card">
        <div class="mb-2 flex items-center justify-between text-xs font-medium uppercase tracking-wide text-muted">
          Docker CPU
          <Cpu class="h-4 w-4 text-accent" />
        </div>
        <div class="text-2xl font-semibold tracking-tight">{{ fmtPct(cpuNow) }}</div>
        <p class="mt-1 text-xs text-muted">Sum of running containers. 100% is one core.</p>
      </div>
      <div class="card">
        <div class="mb-2 flex items-center justify-between text-xs font-medium uppercase tracking-wide text-muted">
          Combined memory
          <MemoryStick class="h-4 w-4 text-accent" />
        </div>
        <div class="text-2xl font-semibold tracking-tight">{{ fmtSize(memNow) }}</div>
        <p class="mt-1 text-xs text-muted">RAM in use across {{ running.length }} running container{{ running.length === 1 ? '' : 's' }}</p>
      </div>
      <div class="card">
        <div class="mb-2 flex items-center justify-between text-xs font-medium uppercase tracking-wide text-muted">
          Network
          <Activity class="h-4 w-4 text-accent" />
        </div>
        <div class="flex items-baseline gap-4">
          <div>
            <div class="flex items-center gap-1 text-lg font-semibold"><ArrowDown class="h-3.5 w-3.5 text-accent" />{{ fmtRate(rxNow) }}</div>
            <div class="text-xs text-muted">In</div>
          </div>
          <div>
            <div class="flex items-center gap-1 text-lg font-semibold"><ArrowUp class="h-3.5 w-3.5 text-accent" />{{ fmtRate(txNow) }}</div>
            <div class="text-xs text-muted">Out</div>
          </div>
        </div>
      </div>
      <div class="card">
        <div class="mb-2 flex items-center justify-between text-xs font-medium uppercase tracking-wide text-muted">
          Containers
          <Box class="h-4 w-4 text-accent" />
        </div>
        <div class="text-2xl font-semibold tracking-tight">{{ running.length }}<span class="text-base font-medium text-muted"> / {{ containers.length }}</span></div>
        <p class="mt-1 text-xs text-muted">
          {{ containers.length - running.length }} stopped
          <span v-if="attention"> · {{ attention }} need attention</span>
        </p>
      </div>
    </div>

    <div class="grid gap-4 xl:grid-cols-3">
      <div class="card xl:col-span-1">
        <h2 class="mb-3 text-sm font-medium text-heading">CPU over time</h2>
        <AreaChart :series="[{ values: cpuHistory, color: '#1ebe8a', fill: true }]" :height="96" />
        <p class="mt-2 text-xs text-muted">Combined CPU of running containers</p>
      </div>
      <div class="card xl:col-span-1">
        <h2 class="mb-3 text-sm font-medium text-heading">Memory over time</h2>
        <AreaChart :series="[{ values: memHistory, color: '#1ebe8a', fill: true }]" :height="96" />
        <p class="mt-2 text-xs text-muted">Combined RAM of running containers</p>
      </div>
      <div class="card xl:col-span-1">
        <h2 class="mb-3 text-sm font-medium text-heading">Network over time</h2>
        <AreaChart
          :series="[
            { values: netHistory.rx, color: '#1ebe8a', fill: true },
            { values: netHistory.tx, color: '#1ebe8a', fill: false },
          ]"
          :height="96"
        />
        <p class="mt-2 text-xs text-muted">In and out</p>
      </div>
    </div>

    <div class="grid gap-4 xl:grid-cols-2">
      <div class="card">
        <h2 class="mb-3 text-sm font-medium text-heading">Top containers by CPU</h2>
        <div v-if="topCpu.length" class="space-y-3">
          <button
            v-for="c in topCpu"
            :key="c.id"
            type="button"
            class="block w-full text-left"
            @click="openContainer(c)"
          >
            <div class="mb-1 flex items-center justify-between gap-3 text-sm">
              <span class="truncate font-medium">{{ c.name }}</span>
              <span class="inline-flex shrink-0 items-center gap-2 text-muted">
                <Sparkline v-if="c.sparkline?.length" :points="c.sparkline" field="cpu_pct" />
                {{ fmtPct(c.cpu_pct) }}
              </span>
            </div>
            <div class="h-1.5 overflow-hidden rounded-full bg-surface-overlay">
              <div class="h-full rounded-full bg-accent" :style="{ width: share(c.cpu_pct, cpuNow) }" />
            </div>
          </button>
        </div>
        <p v-else class="text-sm text-muted">No running containers.</p>
      </div>

      <div class="card">
        <h2 class="mb-3 text-sm font-medium text-heading">Top containers by memory</h2>
        <div v-if="topMem.length" class="space-y-3">
          <button
            v-for="c in topMem"
            :key="c.id"
            type="button"
            class="block w-full text-left"
            @click="openContainer(c)"
          >
            <div class="mb-1 flex items-center justify-between gap-3 text-sm">
              <span class="truncate font-medium">{{ c.name }}</span>
              <span class="inline-flex shrink-0 items-center gap-2 text-muted">
                <Sparkline v-if="c.sparkline?.length" :points="c.sparkline" field="mem_bytes" />
                {{ fmtSize(c.mem_bytes || 0) }}
              </span>
            </div>
            <div class="h-1.5 overflow-hidden rounded-full bg-surface-overlay">
              <div class="h-full rounded-full bg-accent" :style="{ width: share(c.mem_bytes, memNow) }" />
            </div>
          </button>
        </div>
        <p v-else class="text-sm text-muted">No running containers.</p>
      </div>
    </div>

    <div class="grid gap-4 lg:grid-cols-3">
      <div class="card">
        <h2 class="mb-3 flex items-center gap-2 text-sm font-medium text-heading">
          <Box class="h-4 w-4 text-muted" /> State
        </h2>
        <div v-if="containers.length" class="mb-3 flex h-2 overflow-hidden rounded-full bg-surface-overlay">
          <div
            v-for="[state, count] in stateCounts"
            :key="state"
            :class="stateColor(state)"
            :style="{ width: `${(count / containers.length) * 100}%` }"
          />
        </div>
        <ul class="space-y-1.5 text-sm">
          <li v-for="[state, count] in stateCounts" :key="state" class="flex justify-between">
            <span class="capitalize text-muted">{{ state }}</span>
            <span>{{ count }}</span>
          </li>
          <li v-if="!stateCounts.length" class="text-muted">No containers.</li>
        </ul>
      </div>

      <div class="card">
        <h2 class="mb-3 flex items-center gap-2 text-sm font-medium text-heading">
          <HeartPulse class="h-4 w-4 text-muted" /> Health
        </h2>
        <ul class="space-y-1.5 text-sm">
          <li class="flex justify-between"><span class="text-muted">Healthy</span><span>{{ healthCounts.healthy }}</span></li>
          <li class="flex justify-between"><span class="text-muted">Unhealthy</span><span :class="healthCounts.unhealthy ? 'text-danger' : ''">{{ healthCounts.unhealthy }}</span></li>
          <li class="flex justify-between"><span class="text-muted">Starting</span><span>{{ healthCounts.starting }}</span></li>
          <li class="flex justify-between"><span class="text-muted">No check</span><span>{{ healthCounts.none }}</span></li>
          <li class="flex justify-between"><span class="text-muted">Restart loops</span><span :class="restartLoops.length ? 'text-danger' : ''">{{ restartLoops.length }}</span></li>
        </ul>
        <div v-if="unhealthy.length || restartLoops.length" class="mt-3 space-y-1 border-t border-default pt-3">
          <button
            v-for="c in flagged.slice(0, 4)"
            :key="`${c.id}-attn`"
            type="button"
            class="flex w-full items-center justify-between gap-2 text-left text-sm"
            @click="openContainer(c)"
          >
            <span class="truncate">{{ c.name }}</span>
            <span class="badge" :class="c.restart_loop ? 'badge-error' : 'badge-warn'">{{ c.restart_loop ? 'restart loop' : 'unhealthy' }}</span>
          </button>
        </div>
      </div>

      <div class="card">
        <h2 class="mb-3 flex items-center gap-2 text-sm font-medium text-heading">
          <AlertTriangle class="h-4 w-4 text-muted" /> Attention
        </h2>
        <ul class="space-y-2 text-sm">
          <li>
            <button type="button" class="flex w-full items-center justify-between" @click="router.push('/updates')">
              <span class="inline-flex items-center gap-2 text-muted"><RefreshCw class="h-3.5 w-3.5" /> Image updates</span>
              <span :class="updates.pending?.length ? 'text-warn-accent' : ''">{{ updates.pending?.length || 0 }}</span>
            </button>
          </li>
          <li class="flex items-center justify-between">
            <span class="text-muted">Scan failures</span>
            <span>{{ updates.failures?.length || 0 }}</span>
          </li>
          <li v-if="logging && logging.state && logging.state !== 'disabled'" class="flex items-center justify-between">
            <span class="text-muted">Log shipping</span>
            <span class="capitalize">{{ logging.state.replaceAll('_', ' ') }}</span>
          </li>
          <li v-if="logging && logging.state && logging.state !== 'disabled'" class="flex items-center justify-between">
            <span class="text-muted">Followed containers</span>
            <span>{{ logging.followed || 0 }}</span>
          </li>
          <li v-if="logging && logging.state !== 'disabled' && (logging.dropped || logging.error_count)" class="flex items-center justify-between">
            <span class="text-muted">Dropped / errors</span>
            <span>{{ logging.dropped || 0 }} / {{ logging.error_count || 0 }}</span>
          </li>
        </ul>
      </div>
    </div>

    <div class="grid gap-4 xl:grid-cols-2">
      <div class="card">
        <h2 class="mb-3 flex items-center gap-2 text-sm font-medium text-heading">
          <Layers class="h-4 w-4 text-muted" /> Projects
        </h2>
        <div v-if="projectRows.length" class="max-h-80 space-y-3 overflow-y-auto pr-1">
          <div v-for="p in projectRows" :key="p.name">
            <div class="mb-1 flex items-center justify-between gap-3 text-sm">
              <span class="truncate">{{ p.name }}</span>
              <span class="shrink-0 text-xs text-muted">{{ p.running }}/{{ p.total }} · {{ fmtPct(p.cpu) }} · {{ fmtSize(p.mem) }}</span>
            </div>
            <div class="h-1.5 overflow-hidden rounded-full bg-surface-overlay">
              <div class="h-full rounded-full bg-accent" :style="{ width: p.total ? `${(p.running / p.total) * 100}%` : '0%' }" />
            </div>
          </div>
        </div>
        <p v-else class="text-sm text-muted">No projects.</p>
      </div>

      <div class="card">
        <h2 class="mb-3 flex items-center gap-2 text-sm font-medium text-heading">
          <Activity class="h-4 w-4 text-muted" /> Busiest network
        </h2>
        <div v-if="talkers.some((c) => c.rx + c.tx > 0)" class="space-y-3">
          <button
            v-for="c in talkers"
            :key="c.id"
            type="button"
            class="flex w-full items-center justify-between gap-3 text-left text-sm"
            @click="openContainer(c)"
          >
            <span class="truncate font-medium">{{ c.name }}</span>
            <span class="shrink-0 text-xs text-muted">↓ {{ fmtRate(c.rx) }} · ↑ {{ fmtRate(c.tx) }}</span>
          </button>
        </div>
        <p v-else class="text-sm text-muted">Waiting for a second stats sample.</p>
      </div>
    </div>

    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <button type="button" class="card text-left" @click="router.push('/images')">
        <div class="mb-2 flex items-center justify-between text-xs font-medium uppercase tracking-wide text-muted">
          Images
          <HardDrive class="h-4 w-4" />
        </div>
        <div class="text-xl font-semibold">{{ images.length }}</div>
        <p class="mt-1 text-xs text-muted">{{ fmtSize(imageBytes) }} total · {{ unusedImages.length }} unused ({{ fmtSize(unusedImageBytes) }})</p>
      </button>
      <button type="button" class="card text-left" @click="router.push('/volumes')">
        <div class="mb-2 flex items-center justify-between text-xs font-medium uppercase tracking-wide text-muted">
          Volumes
          <HardDrive class="h-4 w-4" />
        </div>
        <div class="text-xl font-semibold">{{ volumes.length }}</div>
        <p class="mt-1 text-xs text-muted">{{ unusedVolumes }} unused</p>
      </button>
      <button type="button" class="card text-left" @click="router.push('/networks')">
        <div class="mb-2 flex items-center justify-between text-xs font-medium uppercase tracking-wide text-muted">
          Networks
          <Network class="h-4 w-4" />
        </div>
        <div class="text-xl font-semibold">{{ networks.length }}</div>
        <p class="mt-1 text-xs text-muted">{{ networkContainers }} container attachments</p>
      </button>
      <button type="button" class="card text-left" @click="router.push('/ports')">
        <div class="mb-2 flex items-center justify-between text-xs font-medium uppercase tracking-wide text-muted">
          Published ports
          <Plug class="h-4 w-4" />
        </div>
        <div class="text-xl font-semibold">{{ ports.length }}</div>
        <p class="mt-1 text-xs text-muted">Host port bindings</p>
      </button>
    </div>

    <div class="grid gap-4 xl:grid-cols-2">
      <div class="card">
        <h2 class="mb-3 flex items-center gap-2 text-sm font-medium text-heading">
          <Timer class="h-4 w-4 text-muted" /> Longest running
        </h2>
        <div v-if="longest.length" class="space-y-2">
          <button
            v-for="c in longest"
            :key="c.id"
            type="button"
            class="flex w-full items-center justify-between gap-3 text-left text-sm"
            @click="openContainer(c)"
          >
            <span class="truncate">{{ c.name }}</span>
            <span class="shrink-0 text-muted">{{ c.uptime || '-' }}</span>
          </button>
        </div>
        <p v-else class="text-sm text-muted">No start times yet.</p>
      </div>
      <div class="card">
        <h2 class="mb-1 text-sm font-medium text-heading">Closest to memory limit</h2>
        <p class="mb-3 text-xs text-muted">Share of each container's limit. With no limit set, that is the host.</p>
        <div v-if="nearLimit.length" class="space-y-3">
          <button
            v-for="c in nearLimit"
            :key="c.id"
            type="button"
            class="block w-full text-left"
            @click="openContainer(c)"
          >
            <div class="mb-1 flex items-center justify-between gap-3 text-sm">
              <span class="truncate">{{ c.name }}</span>
              <span class="text-muted">{{ fmtPct(c.mem_pct) }}</span>
            </div>
            <div class="h-1.5 overflow-hidden rounded-full bg-surface-overlay">
              <div
                class="h-full rounded-full"
                :class="c.mem_pct >= 80 ? 'bg-amber-400' : 'bg-accent'"
                :style="{ width: `${Math.min(100, num(c.mem_pct))}%` }"
              />
            </div>
          </button>
        </div>
        <p v-else class="text-sm text-muted">No memory samples yet.</p>
      </div>
    </div>

    <div v-if="restarts.length" class="card">
      <h2 class="mb-3 text-sm font-medium text-heading">Restart counts</h2>
      <div class="grid gap-2 sm:grid-cols-2">
        <button
          v-for="c in restarts"
          :key="c.id"
          type="button"
          class="flex items-center justify-between gap-3 text-left text-sm"
          @click="openContainer(c)"
        >
          <span class="truncate">{{ c.name }}</span>
          <span class="text-muted">{{ c.restart_count }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
