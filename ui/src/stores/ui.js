import { defineStore } from 'pinia'

export const useUiStore = defineStore('ui', {
  state: () => ({
    message: '',
    tone: 'error',
  }),
  actions: {
    setError(msg) {
      this.message = msg || ''
      this.tone = 'error'
    },
    setInfo(msg) {
      this.message = msg || ''
      this.tone = 'info'
    },
    clear() {
      this.message = ''
    },
  },
})
