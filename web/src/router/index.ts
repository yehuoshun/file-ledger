import { createRouter, createWebHistory } from 'vue-router'
import RegistryView from '../views/Registry.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'registry',
      component: RegistryView,
    },
  ],
})

export default router