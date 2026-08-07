<script setup>
import { ref, computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import {
  Menu, X, LayoutDashboard, Layers, Image, HardDrive, Network, Plug, LogOut, Sun, Moon,
} from 'lucide-vue-next'
import { useEngineStore } from '@/stores/engine'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'

const route = useRoute()
const store = useEngineStore()
const auth = useAuthStore()
const theme = useThemeStore()
const sidebarOpen = ref(false)

const nav = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard, match: (p) => p === '/' || p.startsWith('/containers/') },
  { to: '/stacks', label: 'Stacks', icon: Layers, match: (p) => p.startsWith('/stacks') },
  { to: '/images', label: 'Images', icon: Image, match: (p) => p.startsWith('/images') },
  { to: '/volumes', label: 'Volumes', icon: HardDrive, match: (p) => p.startsWith('/volumes') },
  { to: '/networks', label: 'Networks', icon: Network, match: (p) => p.startsWith('/networks') },
  { to: '/ports', label: 'Ports', icon: Plug, match: (p) => p.startsWith('/ports') },
]

const pageTitle = computed(() => {
  if (route.name === 'stack') return route.params.name
  if (route.name === 'container') return 'Container'
  const item = nav.find((n) => n.match(route.path))
  return item?.label || 'Dockyard'
})

function onToggleTheme() {
  theme.toggle()
}

async function logout() {
  await auth.logout()
  window.location.href = '/login'
}
</script>

<template>
  <div class="flex h-screen overflow-hidden bg-surface">
    <div v-if="sidebarOpen" class="fixed inset-0 z-40 bg-black/50 lg:hidden" @click="sidebarOpen = false" />

    <aside
      class="fixed inset-y-0 left-0 z-50 flex h-screen w-64 shrink-0 flex-col border-r border-default bg-surface-raised transition-transform lg:static lg:translate-x-0"
      :class="sidebarOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <div class="flex items-center gap-3 border-b border-default p-4">
        <img src="/favicon.png" alt="" class="h-9 w-9 shrink-0 rounded-lg" width="36" height="36" />
        <div class="min-w-0 flex-1">
          <div class="truncate text-sm font-semibold text-heading">Dockyard</div>
          <div class="text-xs text-muted">Docker manager</div>
        </div>
        <button type="button" class="text-muted lg:hidden" @click="sidebarOpen = false"><X class="h-5 w-5" /></button>
      </div>

      <nav class="flex-1 overflow-y-auto p-2">
        <RouterLink
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          class="mb-0.5 flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition"
          :class="item.match(route.path) ? 'nav-item-active' : 'nav-item-inactive'"
          @click="sidebarOpen = false"
        >
          <component :is="item.icon" class="h-4 w-4 shrink-0" />
          {{ item.label }}
        </RouterLink>
      </nav>

      <div class="border-t border-default p-3">
        <div class="mb-2 flex items-center gap-2 px-1 text-xs">
          <span :class="store.connected ? 'text-accent' : 'text-amber-500 dark:text-amber-400'">●</span>
          <span class="text-muted">{{ store.connected ? 'Live updates' : 'Reconnecting…' }}</span>
        </div>
        <div class="flex items-stretch gap-1.5">
          <button type="button" class="btn-secondary shrink-0 px-2.5 py-2" @click="onToggleTheme">
            <Sun v-if="theme.dark" class="h-4 w-4" />
            <Moon v-else class="h-4 w-4" />
          </button>
          <button
            v-if="auth.authRequired"
            type="button"
            class="btn-secondary min-w-0 flex-1 gap-1.5 px-2.5 py-2 text-xs"
            @click="logout"
          >
            <LogOut class="h-4 w-4 shrink-0" />
            <span>Sign out</span>
          </button>
        </div>
      </div>
    </aside>

    <div class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <header class="flex shrink-0 items-center gap-3 border-b border-default bg-surface-raised px-4 py-3 lg:hidden">
        <button type="button" class="text-muted" @click="sidebarOpen = true"><Menu class="h-5 w-5" /></button>
        <span class="truncate font-semibold text-heading">{{ pageTitle }}</span>
      </header>
      <main class="min-h-0 flex-1 overflow-y-auto p-4 md:p-6 text-heading">
        <slot />
      </main>
    </div>
  </div>
</template>
