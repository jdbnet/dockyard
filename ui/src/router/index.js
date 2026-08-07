import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import Dashboard from '@/views/Dashboard.vue'
import ContainerDetail from '@/views/ContainerDetail.vue'
import Stacks from '@/views/Stacks.vue'
import StackDetail from '@/views/StackDetail.vue'
import Images from '@/views/Images.vue'
import Volumes from '@/views/Volumes.vue'
import Networks from '@/views/Networks.vue'
import Ports from '@/views/Ports.vue'
import Login from '@/views/Login.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: Login, meta: { public: true } },
    { path: '/', name: 'dashboard', component: Dashboard },
    { path: '/containers/:id', name: 'container', component: ContainerDetail },
    { path: '/stacks', name: 'stacks', component: Stacks },
    { path: '/stacks/:name', name: 'stack', component: StackDetail },
    { path: '/images', name: 'images', component: Images },
    { path: '/volumes', name: 'volumes', component: Volumes },
    { path: '/networks', name: 'networks', component: Networks },
    { path: '/ports', name: 'ports', component: Ports },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.checked) {
    try {
      await auth.check()
    } catch {
      auth.checked = true
    }
  }

  if (to.meta.public) {
    if (auth.authenticated && to.path === '/login') {
      return '/'
    }
    return true
  }

  if (auth.authRequired && !auth.authenticated) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }

  return true
})

export default router
