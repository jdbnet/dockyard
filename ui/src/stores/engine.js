import { defineStore } from 'pinia'
import { getContainers, wsURL } from '@/api/client'

export const useEngineStore = defineStore('engine', {
  state: () => ({
    composeProjects: [],
    flatView: false,
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
      this.fetch()
    },
  },
})
