<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../shared/http'
import { fmtTime, pct } from '../../shared/format'

const router = useRouter()
const data = ref(null)
const loading = ref(true)
const refreshedAt = ref('')

async function load() {
  loading.value = true
  try {
    data.value = await api.get('/api/v1/dashboard')
    refreshedAt.value = fmtTime(new Date())
  } finally {
    loading.value = false
  }
}

onMounted(load)

const ticks = (rate) => {
  const on = Math.round(Math.max(0, Math.min(1, rate)) * 20)
  return Array.from({ length: 20 }, (_, i) => i < on)
}

const metrics = computed(() => {
  const d = data.value || {}
  return [
    {
      key: 'learn',
      name: '学习完成率',
      value: pct(d.learn_completion),
      raw: d.learn_completion || 0,
      note: `${d.learn_completed || 0} / ${d.learn_expected || 0} 人次`,
    },
    {
      key: 'exam',
      name: '考试通过率',
      value: pct(d.exam_pass_rate),
      raw: d.exam_pass_rate || 0,
      note: `${d.exam_passed || 0} / ${d.exam_submitted || 0} 人`,
    },
    {
      key: 'broadcast',
      name: '宣贯签收率',
      value: pct(d.broadcast_ack_rate),
      raw: d.broadcast_ack_rate || 0,
      note: `${d.broadcast_ack || 0} / ${d.broadcast_total || 0} 人次`,
    },
  ]
})
</script>

<template>
  <div v-loading="loading">
    <div
      v-if="data && (data.issues_open > 0 || data.issues_overdue > 0)"
      class="alert-strip"
      @click="router.push('/issues')"
      style="cursor: pointer"
    >
      <span class="a-label">未闭环 {{ data.issues_open }} 条</span>
      <span class="a-sub">
        其中超时 {{ data.issues_overdue }} 条<span v-if="data.issues_overdue > 0"> · 需要今天处理</span>
      </span>
      <span class="a-go">去处理 →</span>
    </div>

    <div class="row-between" style="margin-bottom: 12px">
      <span class="muted">全矿安全运行快照</span>
      <span class="muted num">{{ refreshedAt ? '数据截至 ' + refreshedAt : '' }}</span>
    </div>

    <div class="kpi-row">
      <div class="kpi">
        <div class="k-label">已发布规程</div>
        <div class="k-value">{{ data?.docs ?? 0 }}</div>
        <div class="k-note">共 {{ data?.chunks ?? 0 }} 条可检索条款</div>
      </div>
      <div class="kpi">
        <div class="k-label">今日问答</div>
        <div class="k-value">{{ data?.qa_today ?? 0 }}</div>
        <div class="k-note">累计 {{ data?.qa_total ?? 0 }} 次 · 兜底 {{ pct(data?.qa_fallback_rate) }}</div>
      </div>
      <div class="kpi">
        <div class="k-label">未闭环问题</div>
        <div class="k-value" :style="data?.issues_overdue ? { color: 'var(--seal)' } : {}">
          {{ data?.issues_open ?? 0 }}
        </div>
        <div class="k-note">其中超时 {{ data?.issues_overdue ?? 0 }} 条</div>
      </div>
    </div>

    <div class="panel" style="margin-top: 16px">
      <h2 class="panel-title">培训与宣贯闭环</h2>
      <div v-for="m in metrics" :key="m.key" class="metric">
        <div class="metric-head">
          <span class="metric-name">{{ m.name }}</span>
          <span class="metric-value">{{ m.value }}</span>
          <span class="metric-note">{{ m.note }}</span>
        </div>
        <div class="metric-scale">
          <i
            v-for="(on, i) in ticks(m.raw)"
            :key="i"
            :class="{ on, alert: on && m.raw < 0.6 }"
          ></i>
        </div>
      </div>
      <div class="muted" style="margin-top: 10px">
        刻度向左为未达标（低于 60% 标朱红），向右为达标。
      </div>
    </div>

    <div class="panel" style="margin-top: 16px">
      <h2 class="panel-title">口径说明</h2>
      <dl class="kv">
        <dt>学习完成率</dt>
        <dd>已完成学习记录 ÷（在册人数 × 已发布学习条目）</dd>
        <dt>考试通过率</dt>
        <dd>通过人数 ÷ 已交卷人数，补考按最新一次成绩计</dd>
        <dt>宣贯签收率</dt>
        <dd>已签收人次 ÷ 已发布宣贯应签收人次</dd>
      </dl>
    </div>
  </div>
</template>
