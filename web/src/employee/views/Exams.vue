<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../shared/http'
import { fmtDate } from '../../shared/format'

const router = useRouter()
const papers = ref([])
const records = ref([])
const loading = ref(true)

const recordMap = computed(() => {
  const m = {}
  records.value.forEach((r) => {
    m[r.paper_id] = r
  })
  return m
})

onMounted(async () => {
  try {
    const [p, r] = await Promise.all([
      api.get('/api/v1/exam/papers', { page_size: 50 }),
      api.get('/api/v1/exam/records', { page_size: 50 }),
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
          <van-tag v-if="recordMap[p.id]?.status !== 'in_progress'" :type="recordMap[p.id]?.passed ? 'success' : 'warning'">
            {{ recordMap[p.id] ? (recordMap[p.id].passed ? '已通过' : '未通过') : '未参加' }}
          </van-tag>
          <van-tag v-else type="primary">进行中</van-tag>
        </div>
        <div class="muted" style="margin-top: 8px">
          及格 {{ p.pass_score }} 分 · 时长 {{ p.duration_min }} 分钟
          <span v-if="p.end_at"> · 截止 {{ fmtDate(p.end_at, true) }}</span>
        </div>
        <div v-if="recordMap[p.id]" class="muted" style="margin-top: 6px">
          得分 {{ recordMap[p.id].score }} ·
          {{ recordMap[p.id].status === 'in_progress' ? '未交卷' : '已交卷' }}
        </div>
        <div style="margin-top: 12px">
          <van-button
            block
            :type="recordMap[p.id] && recordMap[p.id].status !== 'in_progress' ? 'default' : 'primary'"
            @click="router.push(`/exams/${p.id}/run`)"
          >
            {{ recordMap[p.id] && recordMap[p.id].status !== 'in_progress' ? '查看成绩' : '进入考试' }}
          </van-button>
        </div>
      </div>
    </div>
  </div>
</template>
