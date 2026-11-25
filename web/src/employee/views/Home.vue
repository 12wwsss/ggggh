<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, getUser } from '../../shared/http'

const router = useRouter()
const user = ref(getUser() || {})
const pendingAck = ref(0)
const papers = ref(0)
const recent = ref([])
const loading = ref(true)

// 头部说的是“几件事要办”，不是场次：考试 5 场也只是 1 件事。
const todoKinds = computed(() => (papers.value > 0 ? 1 : 0) + (pendingAck.value > 0 ? 1 : 0))

const presets = [
  { label: '瓦斯超限怎么办', q: '瓦斯浓度达到1.5%时怎么办？' },
  { label: '停风了怎么办', q: '停风时怎么处理？' },
  { label: '透水征兆有哪些', q: '出现透水征兆怎么办？' },
  { label: '敲帮问顶怎么做', q: '敲帮问顶应该怎么做？' },
]

onMounted(async () => {
  try {
    const [mine, examList, logs] = await Promise.all([
      api.get('/api/v1/broadcast/mine', { pending: true }),
      api.get('/api/v1/exam/papers', { page_size: 5 }),
      api.get('/api/v1/qa/logs', { page_size: 3 }),
    ])
    pendingAck.value = (mine || []).length
    papers.value = (examList.list || []).length
    recent.value = (logs.list || []).slice(0, 3)
  } catch (e) {
    console.warn(e.message)
  } finally {
    loading.value = false
  }
})

function ask(q) {
  router.push({ path: '/ask', query: q ? { q } : {} })
}
</script>

<template>
  <div class="page">
    <header class="lt-hero">
      <h1 class="lt-name">{{ user.name || '安全员' }}，欢迎回来</h1>
      <p class="lt-meta">
        {{ user.dept_name || '矿部' }} · {{ user.post || '员工' }}
        <span v-if="todoKinds"> · 今天有 {{ todoKinds }} 件事要办</span>
        <span v-else> · 今天暂无待办</span>
      </p>
    </header>

    <div class="pad">
      <button class="lt-btn lt-primary" @click="ask()">
        问规程<span class="lt-sub">点一下就能问</span>
      </button>

      <div class="lt-row">
        <button class="lt-btn" @click="router.push('/issues/new')">报隐患</button>
        <button class="lt-btn" @click="router.push('/learn')">学规程</button>
      </div>

      <button class="lt-btn" style="margin-top: 8px" @click="router.push('/media')">
        看视频<span class="lt-sub">操作示范、事故案例</span>
      </button>

      <div class="lt-card">
        <div class="lt-card-head">
          <span class="lt-tag">今天要做</span>
          <span class="lt-count">{{ loading ? '统计中…' : todoKinds + ' 件' }}</span>
        </div>

        <button v-if="papers" class="lt-todo" @click="router.push('/exams')">
          <span class="lt-todo-main">参加考试</span>
          <span class="lt-todo-side">可考 {{ papers }} 场 →</span>
        </button>
        <button v-if="pendingAck" class="lt-todo" @click="router.push('/broadcasts')">
          <span class="lt-todo-main">宣贯签收</span>
          <span class="lt-todo-side">{{ pendingAck }} 条待签收 →</span>
        </button>
        <div v-if="!papers && !pendingAck && !loading" class="lt-empty">
          今天没有待办，可以查条款
        </div>
      </div>

      <div class="lt-sec">常问的（点一下就行）</div>
      <div class="lt-chips">
        <button v-for="p in presets" :key="p.label" class="lt-chip" @click="ask(p.q)">
          {{ p.label }}
        </button>
      </div>

      <div v-if="recent.length" class="lt-recent">
        <div class="lt-sec">上次问过</div>
        <button v-for="log in recent" :key="log.id" class="lt-recent-row" @click="ask(log.question)">
          <span class="lt-recent-q">{{ log.question }}</span>
          <span class="lt-recent-go">再问 →</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page {
  padding-bottom: 92px; /* 给固定 tabbar 留安全区，避免最后一行被压住 */
}
.lt-hero {
  background: var(--ink);
  color: var(--paper-2);
  padding: 26px 18px 30px;
}
.lt-name {
  font-family: var(--font-serif);
  font-size: 30px;
  margin: 0 0 8px;
  letter-spacing: 0.02em;
}
.lt-meta {
  margin: 0;
  font-size: 17px;
  line-height: 1.6;
  opacity: 0.75;
}

.lt-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  width: 100%;
  min-height: 64px;
  padding: 10px 16px;
  border: 2px solid var(--ink);
  border-radius: 4px;
  background: var(--sheet);
  color: var(--ink);
  font-size: 24px;
  font-weight: 700;
  letter-spacing: 0.04em;
}
.lt-btn:active {
  transform: translateY(1px);
}
.lt-primary {
  background: var(--seal);
  border-color: var(--seal);
  color: #fbf6ec;
  margin-bottom: 14px;
}
.lt-sub {
  font-size: 16px;
  font-weight: 400;
  opacity: 0.85;
}

.lt-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-bottom: 16px;
}
.lt-row .lt-btn {
  min-height: 58px;
  font-size: 21px;
}

.lt-card {
  border: 2px solid var(--ink);
  border-radius: 4px;
  background: var(--sheet);
  padding: 16px;
  margin-bottom: 20px;
}
.lt-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.lt-tag {
  border: 2px solid var(--seal);
  color: var(--seal);
  font-size: 15px;
  letter-spacing: 0.15em;
  padding: 2px 8px;
}
.lt-count {
  font-size: 16px;
  color: var(--ink-3);
  font-variant-numeric: tabular-nums;
}
.lt-todo {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  min-height: 60px;
  padding: 6px 2px;
  background: transparent;
  border: 0;
  border-top: 1px solid var(--rule);
  font-size: 21px;
  color: var(--ink);
  text-align: left;
}
.lt-todo-main {
  font-weight: 600;
}
.lt-todo-side {
  font-size: 17px;
  color: var(--seal);
}
.lt-empty {
  font-size: 18px;
  color: var(--ink-3);
  padding: 6px 0;
}

.lt-sec {
  font-size: 17px;
  color: var(--ink-2);
  letter-spacing: 0.08em;
  margin-bottom: 12px;
}
.lt-chips {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.lt-chip {
  min-height: 60px;
  padding: 12px;
  border: 2px solid var(--rule-strong);
  border-radius: 4px;
  background: var(--sheet);
  color: var(--ink);
  font-size: 19px;
  line-height: 1.4;
  text-align: center;
}
.lt-chip:active {
  border-color: var(--seal);
  color: var(--seal);
}

.lt-recent {
  margin-top: 24px;
}
.lt-recent-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-height: 58px;
  padding: 8px 2px;
  border: 0;
  border-top: 1px solid var(--rule);
  background: transparent;
  text-align: left;
  color: var(--ink);
}
.lt-recent-q {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 18px;
}
.lt-recent-go {
  flex: 0 0 auto;
  font-size: 17px;
  color: var(--seal);
}
</style>
