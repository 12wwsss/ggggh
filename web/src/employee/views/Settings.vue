<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showConfirmDialog, showToast } from 'vant'
import { api, clearSession, getUser } from '../../shared/http'

const router = useRouter()
const qs = reactive([
  {
    q: '登录不上怎么办？',
    a: '先确认账号与口令；首次登录必须修改初始口令。仍失败请联系安全科或管理员重置。',
  },
  {
    q: '为什么问答提示"资料中未找到明确依据"？',
    a: '说明知识库里没有相关条款。可以点「转人工」提交给安全科，或通知管理员补充规程文件。',
  },
  {
    q: '浏览器提示"不安全"？',
    a: '当前通过 IP 访问、还没配 HTTPS，属于正常现象；正式上线会配置域名与证书。',
  },
  {
    q: '考试成绩怎么看解析？',
    a: '进入「考试」→ 对应试卷 →「查看成绩 / 再考一次」页面下方就是「答案与解析」，可按历次成绩切换。',
  },
  {
    q: '手机上能用吗？',
    a: '可以，用手机浏览器直接打开同一地址即可；小程序版本也在同步更新中。',
  },
])
const open = ref({})
const health = ref('')
const version = 'v1.1'

onMounted(async () => {
  try {
    const r = await api.get('/api/health')
    health.value = r.ok ? `正常（模型 ${r.llm ? '在线' : '离线'}）` : '异常'
  } catch {
    health.value = '连不上服务器'
  }
})

function toggle(i) {
  open.value = { ...open.value, [i]: !open.value[i] }
}

async function clearLocal() {
  try {
    await showConfirmDialog({ title: '清除本地数据', message: '会清除本机的登录状态与缓存，需要重新登录。' })
  } catch {
    return
  }
  clearSession()
  localStorage.clear()
  showToast('已清除')
  router.replace('/login')
}

async function logout() {
  try {
    await showConfirmDialog({ title: '退出登录', message: '确定退出当前账号吗？' })
  } catch {
    return
  }
  clearSession()
  router.replace('/login')
}
</script>

<template>
  <div class="page">
    <van-nav-bar title="帮助与设置" left-arrow @click-left="$router.back()" />
    <div class="pad">
      <div class="card">
        <div class="row-between">
          <strong>服务状态</strong>
          <span :style="health.startsWith('正常') ? 'color: var(--moss)' : 'color: var(--seal)'">{{ health }}</span>
        </div>
        <div class="muted" style="margin-top: 6px">
          账号：{{ (getUser() || {}).username || '—' }} · 版本 {{ version }}
        </div>
      </div>

      <div class="muted" style="margin: 14px 0 6px">常见问题</div>
      <div v-for="(item, i) in qs" :key="i" class="card" style="cursor: pointer" @click="toggle(i)">
        <div class="row-between">
          <strong>{{ item.q }}</strong>
          <span class="muted">{{ open[i] ? '收起' : '展开' }}</span>
        </div>
        <div v-if="open[i]" class="muted" style="margin-top: 8px; line-height: 1.7">{{ item.a }}</div>
      </div>

      <div class="muted" style="margin: 14px 0 6px">操作</div>
      <div class="card" style="cursor: pointer" @click="router.push('/me')">
        <div class="row-between"><strong>修改口令</strong><span class="muted">›</span></div>
      </div>
      <div class="card" style="cursor: pointer" @click="clearLocal">
        <div class="row-between"><strong>清除本地数据</strong><span class="muted">›</span></div>
      </div>
      <van-button block plain type="danger" style="margin-top: 16px" @click="logout">退出登录</van-button>
    </div>
  </div>
</template>
