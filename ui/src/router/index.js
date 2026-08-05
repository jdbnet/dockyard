import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from '@/views/Dashboard.vue'
import ContainerDetail from '@/views/ContainerDetail.vue'
import Stacks from '@/views/Stacks.vue'
import StackDetail from '@/views/StackDetail.vue'
import Images from '@/views/Images.vue'
import Volumes from '@/views/Volumes.vue'
import Networks from '@/views/Networks.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'dashboard', component: Dashboard },
    { path: '/containers/:id', name: 'container', component: ContainerDetail },
    { path: '/stacks', name: 'stacks', component: Stacks },
    { path: '/stacks/:name', name: 'stack', component: StackDetail },
    { path: '/images', name: 'images', component: Images },
    { path: '/volumes', name: 'volumes', component: Volumes },
    { path: '/networks', name: 'networks', component: Networks },
  ],
})

export default router
