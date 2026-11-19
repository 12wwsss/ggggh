import { createRouter, createWebHistory } from 'vue-router'
import { clearSession, getToken, getUser } from '../shared/http'

const routes = [
  { path: '/login', name: 'login', component: () => import('./views/Login.vue'), meta: { public: true } },
  {
    path: '/',
    component: () => import('./views/Layout.vue'),
    children: [
      { path: '', redirect: '/dashboard' },
      { path: 'dashboard', name: 'dashboard', component: () => import('./views/Dashboard.vue'), meta: { title: '运行看板' } },
      { path: 'docs', name: 'docs', component: () => import('./views/Docs.vue'), meta: { title: '文档与条款' } },
      { path: 'docs/:id', name: 'doc-detail', component: () => import('./views/DocDetail.vue'), meta: { title: '文档详情' } },
      { path: 'qa', name: 'qa', component: () => import('./views/QaLogs.vue'), meta: { title: '问答留痕' } },
      { path: 'learn', name: 'learn', component: () => import('./views/Learn.vue'), meta: { title: '学习与题库' } },
      { path: 'exams', name: 'exams', component: () => import('./views/Exams.vue'), meta: { title: '考试管理' } },
      { path: 'broadcasts', name: 'broadcasts', component: () => import('./views/Broadcasts.vue'), meta: { title: '宣贯管理' } },
      { path: 'issues', name: 'issues', component: () => import('./views/Issues.vue'), meta: { title: '问题受理与闭环' } },
      { path: 'org', name: 'org', component: () => import('./views/Org.vue'), meta: { title: '组织与权限' } },
      { path: 'system', name: 'system', component: () => import('./views/System.vue'), meta: { title: '任务与审计' } },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
]

const router = createRouter({
  history: createWebHistory('/admin/'),
  routes,
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach((to) => {
  const user = getUser()
  // 员工账号不允许进入后台：直接清掉令牌，避免和登录页互相重定向。
  if (user && user.role === 'employee') {
    if (!to.meta.public) {
      clearSession()
      return { name: 'login' }
    }
    clearSession()
    return true
  }
  if (!to.meta.public && !getToken()) return { name: 'login' }
  if (to.name === 'login' && getToken()) return { name: 'dashboard' }
  return true
})

export default router
