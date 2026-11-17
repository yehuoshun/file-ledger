import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from '../views/Dashboard.vue'
import CloudView from '../views/Cloud.vue'
import SettingsView from '../views/Settings.vue'
import TagSettingsView from '../views/TagSettings.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/dashboard',
    },
    { path: '/dashboard', name: 'dashboard', component: DashboardView },
    { path: '/cloud', name: 'cloud', component: CloudView },
    // 设置分两个子页：侧边栏「设置」是 el-sub-menu，这里用扁平路由（不套嵌套 router-view）
    { path: '/settings', redirect: '/settings/general' },
    { path: '/settings/general', name: 'settings-general', component: SettingsView },
    { path: '/settings/tags', name: 'settings-tags', component: TagSettingsView },
  ],
})

export default router
