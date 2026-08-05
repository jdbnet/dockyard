<script setup>
import { ref, computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import {
  Menu, X, LayoutDashboard, Layers, Image, HardDrive, Network, LogOut,
} from 'lucide-vue-next'
import { useEngineStore } from '@/stores/engine'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const store = useEngineStore()
const auth = useAuthStore()
const sidebarOpen = ref(false)

const nav = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard, match: (p) => p === '/' || p.startsWith('/containers/') },
  { to: '/stacks', label: 'Stacks', icon: Layers, match: (p) => p.startsWith('/stacks') },
  { to: '/images', label: 'Images', icon: Image, match: (p) => p.startsWith('/images') },
  { to: '/volumes', label: 'Volumes', icon: HardDrive, match: (p) => p.startsWith('/volumes') },
  { to: '/networks', label: 'Networks', icon: Network, match: (p) => p.startsWith('/networks') },
]

const pageTitle = computed(() => {
  const item = nav.find((n) => n.match(route.path))
  return item?.label || 'Dockyard'
})

async function logout() {
  await auth.logout()
  window.location.href = '/login'
}
</script>

<template>
  <div class="flex h-screen overflow-hidden bg-canvas-dark">
    <div v-if="sidebarOpen" class="fixed inset-0 z-40 bg-black/50 lg:hidden" @click="sidebarOpen = false" />

    <aside
      class="fixed inset-y-0 left-0 z-50 flex h-screen w-64 shrink-0 flex-col border-r border-slate-800 bg-canvas-subtle-dark transition-transform lg:static lg:translate-x-0"
      :class="sidebarOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <div class="flex items-center gap-3 border-b border-slate-800 p-4">
        <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-accent/15 text-accent font-bold">D</div>
        <div class="min-w-0 flex-1">
          <div class="truncate text-sm font-semibold">Dockyard</div>
          <div class="text-xs text-slate-500">Docker manager</div>
        </div>
        <button class="lg:hidden text-slate-400" @click="sidebarOpen = false"><X class="h-5 w-5" /></button>
      </div>

      <nav class="flex-1 overflow-y-auto p-2">
        <RouterLink
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          class="mb-0.5 flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition"
          :class="item.match(route.path)
            ? 'bg-accent/15 font-medium text-accent'
            : 'text-slate-400 hover:bg-canvas-inset-dark hover:text-slate-200'"
          @click="sidebarOpen = false"
        >
          <component :is="item.icon" class="h-4 w-4 shrink-0" />
          {{ item.label }}
        </RouterLink>
      </nav>

      <div class="border-t border-slate-800 p-3">
        <div class="mb-2 flex items-center gap-2 px-3 text-xs">
          <span :class="store.connected ? 'text-accent' : 'text-amber-400'">●</span>
          <span class="text-slate-500">{{ store.connected ? 'Live updates' : 'Reconnecting…' }}</span>
        </div>
        <button
          v-if="auth.authRequired"
          class="btn-ghost flex w-full items-center justify-center gap-2 text-sm"
          @click="logout"
        >
          <LogOut class="h-4 w-4" /> Sign out
        </button>
      </div>
    </aside>

    <div class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <header class="flex shrink-0 items-center gap-3 border-b border-slate-800 bg-canvas-subtle-dark px-4 py-3 lg:hidden">
        <button class="text-slate-300" @click="sidebarOpen = true"><Menu class="h-5 w-5" /></button>
        <span class="truncate font-semibold">{{ pageTitle }}</span>
      </header>
      <header class="hidden shrink-0 items-center border-b border-slate-800 bg-canvas-subtle-dark px-6 py-3 lg:flex">
        <span class="font-semibold text-slate-200">{{ pageTitle }}</span>
      </header>
      <main class="min-h-0 flex-1 overflow-y-auto p-4 md:p-6">
        <slot />
      </main>
    </div>
  </div>
</template>
