import { createApp } from 'vue'
import Vant from 'vant'
import 'vant/lib/index.css'
import '../shared/theme.css'
import '../shared/large'
import App from './App.vue'
import router from './router'
import { onUnauthorized, saveSession, clearSession } from '../shared/http'

// 小程序 web-view 打开时带一次性票据：?ticket=xxx
// 用票据换会话，避免把 JWT 放进 URL 被网关/日志记录。
async function bootstrapTicket() {
  const ticket = window.__COALRAG_TICKET__
  if (!ticket) return
  delete window.__COALRAG_TICKET__
  try {
    const res = await fetch('/api/v1/auth/ticket', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ code: ticket }),
    })
    const body = await res.json()
    if (res.ok && body?.data?.token) {
      saveSession(body.data.token, body.data.user)
    } else {
      clearSession()
    }
  } catch {
    // 网络异常时保留原有会话，交给后续页面处理
  }
}

onUnauthorized(() => {
  if (router.currentRoute.value.name !== 'login') router.replace('/login')
})

bootstrapTicket().finally(() => {
  createApp(App).use(Vant).use(router).mount('#app')
})
