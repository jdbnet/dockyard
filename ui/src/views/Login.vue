<script setup>
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

function isSafeRedirect(path) {
  return typeof path === 'string' && path.startsWith('/') && !path.startsWith('//')
}

async function login() {
  error.value = ''
  loading.value = true
  try {
    await auth.login(username.value, password.value)
    const redirect = route.query.redirect
    router.push(isSafeRedirect(redirect) ? redirect : '/')
  } catch {
    error.value = 'Invalid username or password'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-canvas-dark px-4">
    <div class="w-full max-w-md rounded-xl border border-slate-800 bg-canvas-subtle-dark p-8 shadow-xl">
      <div class="mb-6 text-center">
        <h1 class="text-2xl font-semibold text-accent">Dockyard</h1>
        <p class="mt-1 text-sm text-slate-500">Sign in to manage Docker</p>
      </div>

      <form class="space-y-4" @submit.prevent="login">
        <div v-if="error" class="rounded-lg border border-red-500/40 bg-red-900/20 px-3 py-2 text-center text-sm text-red-300">
          {{ error }}
        </div>
        <div>
          <label class="mb-1 block text-sm text-slate-400">Username</label>
          <input v-model="username" type="text" required autofocus class="input-field" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-slate-400">Password</label>
          <input v-model="password" type="password" required class="input-field" />
        </div>
        <button type="submit" class="btn-primary w-full py-2" :disabled="loading">
          {{ loading ? 'Signing in…' : 'Sign in' }}
        </button>
      </form>
    </div>
  </div>
</template>
