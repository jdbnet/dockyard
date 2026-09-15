<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { execWSURL } from '@/api/client'

const props = defineProps({
  containerId: { type: String, required: true },
  active: { type: Boolean, default: false },
  running: { type: Boolean, default: false },
})

const host = ref(null)
let term = null
let fitAddon = null
let ws = null
let resizeObserver = null

function sendResize() {
  if (!ws || ws.readyState !== WebSocket.OPEN || !fitAddon) return
  fitAddon.fit()
  const { cols, rows } = term
  ws.send(JSON.stringify({ type: 'resize', cols, rows }))
}

function connect() {
  disconnect()
  if (!props.active || !props.running || !props.containerId || !term) return

  term.clear()
  term.writeln('Connecting…')

  ws = new WebSocket(execWSURL(props.containerId))
  ws.binaryType = 'arraybuffer'

  ws.onopen = () => {
    term.clear()
    sendResize()
  }

  ws.onmessage = (ev) => {
    if (typeof ev.data === 'string') {
      if (ev.data.startsWith('error:')) {
        term.writeln(`\r\n\x1b[31m${ev.data.slice(6).trim()}\x1b[0m`)
      } else {
        term.write(ev.data)
      }
      return
    }
    const bytes = ev.data instanceof ArrayBuffer ? new Uint8Array(ev.data) : new Uint8Array(ev.data)
    term.write(bytes)
  }

  ws.onclose = () => {
    term.writeln('\r\n\x1b[90mSession ended.\x1b[0m')
  }

  ws.onerror = () => {
    term.writeln('\r\n\x1b[31mConnection failed.\x1b[0m')
  }
}

function disconnect() {
  ws?.close()
  ws = null
}

function initTerminal() {
  if (!host.value || term) return
  term = new Terminal({
    cursorBlink: true,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
    fontSize: 13,
    theme: {
      background: '#0f1419',
      foreground: '#e2e8f0',
      cursor: '#1ebe8a',
    },
  })
  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(host.value)
  fitAddon.fit()

  term.onData((data) => {
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    ws.send(new TextEncoder().encode(data))
  })

  resizeObserver = new ResizeObserver(() => {
    if (!props.active) return
    sendResize()
  })
  resizeObserver.observe(host.value)
}

onMounted(() => {
  initTerminal()
  if (props.active && props.running) connect()
})

onUnmounted(() => {
  disconnect()
  resizeObserver?.disconnect()
  term?.dispose()
  term = null
})

watch(() => [props.active, props.running, props.containerId], () => {
  if (props.active && props.running) {
    connect()
  } else {
    disconnect()
    if (!props.running && term) {
      term.clear()
      term.writeln('\x1b[90mShell is only available for running containers.\x1b[0m')
    }
  }
})
</script>

<template>
  <div ref="host" class="terminal-panel" />
</template>

<style scoped>
.terminal-panel {
  min-height: 20rem;
  border-radius: 0.5rem;
  overflow: hidden;
}
</style>
