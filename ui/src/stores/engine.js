import { defineStore } from 'pinia'
import { getContainers, wsURL } from '@/api/client'

const FLAT_VIEW_KEY = 'dockyard-flat-view'

function readFlatView() {
  return localStorage.getItem(FLAT_VIEW_KEY) === 'true'
}

function writeFlatView(flat) {
  localStorage.setItem(FLAT_VIEW_KEY, flat ? 'true' : 'false')
}

export const useEngineStore = defineStore('engine', {
  state: () => ({
    composeProjects: [],
    flatView: readFlatView(),
    connected: false,
    lastEvent: null,
    eventsWS: null,
  }),
  actions: {
    async fetch() {
      if (this.flatView) {
        const containers = await getContainers(true)
        this.composeProjects = [{ name: 'All containers', containers }]
      } else {
        this.composeProjects = await getContainers(false)
      }
    },
    connectEvents() {
      if (this.eventsWS) return
      const ws = new WebSocket(wsURL('/ws/events'))
      this.eventsWS = ws
      ws.onopen = () => { this.connected = true }
      ws.onclose = () => {
        this.connected = false
        this.eventsWS = null
        setTimeout(() => this.connectEvents(), 3000)
      }
      ws.onmessage = (ev) => {
        this.lastEvent = JSON.parse(ev.data)
        this.fetch()
      }
    },
    toggleFlat() {
      this.flatView = !this.flatView
      writeFlatView(this.flatView)
      this.fetch()
    },
  },
})
