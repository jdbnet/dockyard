<script setup>
import { computed, ref, watch } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import AppLayout from '@/components/AppLayout.vue'
import { useEngineStore } from '@/stores/engine'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const store = useEngineStore()
const auth = useAuthStore()
const engineStarted = ref(false)

const isPublic = computed(() => route.meta.public)

function connectEngine() {
  if (engineStarted.value) return
  engineStarted.value = true
  store.fetch()
  store.connectEvents()
}

watch(
  [() => auth.authenticated, isPublic],
  ([ok, publicRoute]) => {
    if (ok && !publicRoute) {
      connectEngine()
    }
  },
  { immediate: true },
)
</script>

<template>
  <AppLayout v-if="!isPublic">
    <RouterView />
  </AppLayout>
  <RouterView v-else />
</template>
