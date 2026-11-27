import { createRouter, createWebHistory } from 'vue-router'
import { getToken } from '../shared/http'

// web-view 带来的 ?ticket= 必须在路由初始化前摘掉，否则 createWebHistory
// 会把票据写回地址栏；票据暂存到 window，由 main.js 换取会话。
;(function stripTicket() {
  const u = new URL(window.location.href)
  const t = u.searchParams.get('ticket')
  if (!t) return
  window.__COALRAG_TICKET__ = t
  u.searchParams.delete('ticket')
  window.history.replaceState({}, '', u.toString())
})()

const routes = [
  { path: '/login', name: 'login', component: () => import('./views/Login.vue'), meta: { public: true } },
  { path: '/', name: 'home', component: () => import('./views/Home.vue'), meta: { tab: true, title: '首页' } },
  { path: '/ask', name: 'ask', component: () => import('./views/Ask.vue'), meta: { title: '问规程' } },
  { path: '/learn', name: 'learn', component: () => import('./views/Learn.vue'), meta: { tab: true, title: '学规程' } },
  { path: '/learn/:id', name: 'learn-detail', component: () => import('./views/LearnDetail.vue'), meta: { title: '学习' } },
  { path: '/quiz/:id', name: 'quiz', component: () => import('./views/Quiz.vue'), meta: { title: '小测' } },
  { path: '/exams', name: 'exams', component: () => import('./views/Exams.vue'), meta: { tab: true, title: '考试' } },
  { path: '/exams/:id/run', name: 'exam-run', component: () => import('./views/ExamRun.vue'), meta: { title: '在线考试' } },
  { path: '/broadcasts', name: 'broadcasts', component: () => import('./views/Broadcasts.vue'), meta: { title: '宣贯签收' } },
  { path: '/culture', name: 'culture', component: () => import('./views/Culture.vue'), meta: { title: '安全文化' } },
  { path: '/culture/:id', name: 'culture-detail', component: () => import('./views/CultureDetail.vue'), meta: { title: '详情' } },
  { path: '/media', name: 'media', component: () => import('./views/Media.vue'), meta: { title: '视频资料' } },
  { path: '/issues', name: 'issues', component: () => import('./views/Issues.vue'), meta: { title: '我的问题' } },
  { path: '/issues/new', name: 'issue-new', component: () => import('./views/IssueNew.vue'), meta: { title: '上报问题' } },
  { path: '/issues/:id', name: 'issue-detail', component: () => import('./views/IssueDetail.vue'), meta: { title: '问题详情' } },
  { path: '/messages', name: 'messages', component: () => import('./views/Messages.vue'), meta: { title: '消息中心' } },
  { path: '/todos', name: 'todos', component: () => import('./views/Todos.vue'), meta: { title: '待办' } },
  { path: '/training', name: 'training', component: () => import('./views/Training.vue'), meta: { title: '新员工带培' } },
  { path: '/wrong', name: 'wrong', component: () => import('./views/Wrong.vue'), meta: { title: '错题本' } },
  { path: '/risks', name: 'risks', component: () => import('./views/Risks.vue'), meta: { title: '岗位风险' } },
  { path: '/me', name: 'me', component: () => import('./views/Me.vue'), meta: { tab: true, title: '我的' } },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory('/'),
  routes,
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach((to) => {
  if (!to.meta.public && !getToken()) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.name === 'login' && getToken()) return { name: 'home' }
  return true
})

export default router
