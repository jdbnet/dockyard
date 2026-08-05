<script setup>
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { onMounted } from 'vue'
import { useEngineStore } from '@/stores/engine'

const route = useRoute()
const store = useEngineStore()

onMounted(() => {
  store.fetch()
  store.connectEvents()
})

const links = [
  { to: '/', label: 'Dashboard' },
  { to: '/stacks', label: 'Stacks' },
  { to: '/images', label: 'Images' },
  { to: '/volumes', label: 'Volumes' },
  { to: '/networks', label: 'Networks' },
]
</script>

<template>
  <div class="min-h-screen bg-canvas-dark">
    <header class="border-b border-slate-800 bg-canvas-subtle-dark">
      <div class="mx-auto flex max-w-7xl items-center justify-between px-4 py-3">
        <div class="flex items-center gap-6">
          <h1 class="text-lg font-semibold text-accent">Dockyard</h1>
          <nav class="flex gap-1">
            <RouterLink
              v-for="link in links"
              :key="link.to"
              :to="link.to"
              class="nav-link"
              :class="{ 'nav-link-active': route.path === link.to }"
            >
              {{ link.label }}
            </RouterLink>
          </nav>
        </div>
        <div class="flex items-center gap-3 text-sm text-slate-500">
          <span :class="store.connected ? 'text-accent' : 'text-amber-400'">
            {{ store.connected ? '● live' : '○ reconnecting' }}
          </span>
        </div>
      </div>
    </header>
    <main class="mx-auto max-w-7xl px-4 py-6">
      <RouterView />
    </main>
  </div>
</template>
