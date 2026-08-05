<script setup>
import { computed, watch } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import AppLayout from '@/components/AppLayout.vue'
import { useEngineStore } from '@/stores/engine'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const store = useEngineStore()
const auth = useAuthStore()

const isPublic = computed(() => route.meta.public)

function connectEngine() {
  store.fetch()
  store.connectEvents()
}

watch(
  () => auth.authenticated,
  (ok) => {
    if (ok && !isPublic.value) connectEngine()
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
