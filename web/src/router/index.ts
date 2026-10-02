import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from '../views/Dashboard.vue'
import CloudView from '../views/Cloud.vue'
import SettingsView from '../views/Settings.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/dashboard',
    },
    { path: '/dashboard', name: 'dashboard', component: DashboardView },
    { path: '/cloud', name: 'cloud', component: CloudView },
    { path: '/settings', name: 'settings', component: SettingsView },
  ],
})

export default router
