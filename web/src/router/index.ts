import { createRouter, createWebHistory } from 'vue-router'
import HomeLayout from '../views/Home.vue'
import DashboardView from '../views/Dashboard.vue'
import SettingsView from '../views/Settings.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/home/dashboard',
    },
    {
      path: '/home',
      component: HomeLayout,
      children: [
        { path: 'dashboard', name: 'dashboard', component: DashboardView },
      ],
    },
    { path: '/settings', name: 'settings', component: SettingsView },
  ],
})

export default router