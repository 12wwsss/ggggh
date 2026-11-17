<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../shared/http'
import { fmtDate } from '../../shared/format'

const router = useRouter()
const papers = ref([])
const records = ref([])
const loading = ref(true)

// 按试卷汇总：已考次数、最近一次记录、是否有未交卷的
const byPaper = computed(() => {
  const m = {}
  for (const r of records.value) {
    let cur = m[r.paper_id]
    if (!cur) {
      cur = m[r.paper_id] = { used: 0, inProgress: false, latest: null }
    }
    cur.used += 1
    if (r.status === 'in_progress') cur.inProgress = true
    if (!cur.latest || new Date(r.started_at) > new Date(cur.latest.started_at)) cur.latest = r
  }
  return m
})

function info(p) {
  const s = byPaper.value[p.id] || { used: 0, inProgress: false, latest: null }
  const max = p.max_attempts || 1
  return { ...s, max, remaining: Math.max(0, max - s.used) }
}

function actionText(p) {
  const i = info(p)
  if (i.inProgress) return '继续考试'
  if (i.remaining > 0) return i.used > 0 ? '再考一次' : '进入考试'
  return '查看成绩'
}

function actionType(p) {
  const i = info(p)
  return i.inProgress || i.remaining > 0 ? 'primary' : 'default'
}

onMounted(async () => {
  try {
    const [p, r] = await Promise.all([
      api.get('/api/v1/exam/papers', { page_size: 50 }),
      api.get('/api/v1/exam/records', { page_size: 100 }),
    ])
    papers.value = p.list || []
    records.value = r.list || []
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page">
    <van-nav-bar title="在线考试" />
    <div class="pad">
      <van-empty v-if="!loading && !papers.length" description="暂无可参加考试" />
      <div v-for="p in papers" :key="p.id" class="card">
        <div class="row-between">
          <strong>{{ p.title }}</strong>
          <van-tag v-if="info(p).inProgress" type="primary">进行中</van-tag>
          <van-tag v-else-if="!info(p).latest" type="warning">未参加</van-tag>
          <van-tag v-else :type="info(p).latest.passed ? 'success' : 'warning'">
            {{ info(p).latest.passed ? '已通过' : '未通过' }}
          </van-tag>
        </div>
        <div class="muted" style="margin-top: 8px">
          及格 {{ p.pass_score }} 分 · 时长 {{ p.duration_min }} 分钟 · 可考 {{ info(p).max }} 次
          <span v-if="p.end_at"> · 截止 {{ fmtDate(p.end_at, true) }}</span>
        </div>
        <div class="muted" style="margin-top: 6px">
          <template v-if="info(p).used">
            已考 {{ info(p).used }} 次 · 剩余 {{ info(p).remaining }} 次 ·
            最近得分 {{ info(p).latest.score }}
          </template>
          <template v-else>还可考 {{ info(p).remaining }} 次</template>
        </div>
        <div style="margin-top: 12px">
          <van-button block :type="actionType(p)" @click="router.push(`/exams/${p.id}/run`)">
            {{ actionText(p) }}
          </van-button>
          <van-button
            v-if="info(p).latest && !info(p).inProgress"
            block
            size="small"
            plain
            style="margin-top: 8px"
            @click="router.push(`/exams/${p.id}/run?view=1`)"
          >
            查看上次成绩
          </van-button>
        </div>
      </div>
    </div>
  </div>
</template>
