<script setup>
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { clearSession, getUser } from '../../shared/http'
import { roleText } from '../../shared/format'

const route = useRoute()
const router = useRouter()
const user = ref(getUser() || {})
const active = computed(() => route.path)

const groups = [
  {
    title: '运行',
    items: [{ path: '/dashboard', label: '运行看板' }, { path: '/reports', label: '报表中心' }],
  },
  {
    title: '知识与学习',
    items: [
      { path: '/docs', label: '文档与条款' },
      { path: '/qa', label: '问答留痕' },
      { path: '/learn', label: '学习与题库' },
      { path: '/exams', label: '考试管理' },
      { path: '/broadcasts', label: '宣贯管理' },
      { path: '/media', label: '视频资料' },
      { path: '/training', label: '新员工带培', roles: ['admin', 'safety', 'dept_leader', 'employee'] },
    ],
  },
  {
    title: '隐患处置',
    items: [{ path: '/issues', label: '问题受理与闭环' }],
  },
  {
    title: '系统',
    items: [
      { path: '/org', label: '组织与权限' },
      { path: '/system', label: '任务与审计' },
    ],
  },
]

// 统一编号，让导航像手册目录
let counter = 0
const numbered = groups.map((g) => ({
  ...g,
  items: g.items.filter((it) => !it.roles || it.roles.includes(user.value.role)).map((it) => ({ ...it, idx: String(++counter).padStart(2, '0') })),
}))

function isActive(path) {
  return active.value === path || active.value.startsWith(path + '/')
}

async function logout() {
  try {
    await ElMessageBox.confirm('确定退出登录？', '提示', { type: 'warning' })
  } catch {
    return
  }
  clearSession()
  router.replace('/login')
}
</script>

<template>
  <div class="admin-shell">
    <aside class="admin-aside">
      <div class="admin-brand">
        <div class="brand-seal">安</div>
        <div>
          <div class="brand-title">煤矿安全员</div>
          <div class="brand-sub">AI 智能体平台 · 管理台</div>
        </div>
      </div>

      <nav class="admin-nav">
        <div v-for="g in numbered" :key="g.title">
          <div class="nav-group">{{ g.title }}</div>
          <router-link
            v-for="it in g.items"
            :key="it.path"
            :to="it.path"
            class="nav-item"
            :class="{ 'is-active': isActive(it.path) }"
          >
            <span class="idx">{{ it.idx }}</span>
            <span>{{ it.label }}</span>
          </router-link>
        </div>
      </nav>
    </aside>

    <div class="admin-main">
      <header class="admin-header">
        <h1 class="admin-title">{{ route.meta.title || '' }}</h1>
        <div class="admin-user">
          <span>{{ user.name }} · {{ roleText[user.role] || user.role }} · {{ user.dept_name || '' }}</span>
          <el-button link type="primary" @click="logout">退出</el-button>
        </div>
      </header>
      <main class="admin-body">
        <router-view />
      </main>
    </div>
  </div>
</template>
