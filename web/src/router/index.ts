import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/Home.vue'
import RegistryView from '../views/Registry.vue'
import SettingsView from '../views/Settings.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/registry', name: 'registry', component: RegistryView },
    { path: '/settings', name: 'settings', component: SettingsView },
  ],
})

export default router