<script setup>
import { computed, onMounted, ref } from 'vue'
import { showToast } from 'vant'
import { api } from '../../shared/http'

const items = ref([])
const loading = ref(true)
const open = ref({})
const job = ref('')

const jobs = computed(() => ['全部', ...new Set(items.value.map((r) => r.job).filter(Boolean))])
const visible = computed(() =>
  job.value && job.value !== '全部' ? items.value.filter((r) => r.job === job.value) : items.value,
)

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/risks')
    items.value = data.items || []
  } catch (e) {
    showToast(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)

function toggle(id) {
  open.value = { ...open.value, [id]: !open.value[id] }
}
</script>

<template>
  <div class="page">
    <van-nav-bar title="岗位风险" left-arrow @click-left="$router.back()" />
    <div class="pad">
      <van-tabs v-if="jobs.length > 2" v-model:active="job" style="margin-bottom: 10px">
        <van-tab v-for="j in jobs" :key="j" :title="j" :name="j" />
      </van-tabs>

      <van-empty v-if="!loading && !visible.length" description="还没有录岗位风险" />
      <div v-for="r in visible" :key="r.id" class="card" style="cursor: pointer" @click="toggle(r.id)">
        <div class="row-between">
          <strong>{{ r.title }}</strong>
          <van-tag :type="r.level === '重大' ? 'danger' : r.level === '较大' ? 'warning' : 'default'">
            {{ r.level || '一般' }}
          </van-tag>
        </div>
        <div class="muted" style="margin-top: 6px">{{ r.job }} · {{ r.cat }}</div>
        <div style="margin-top: 8px">{{ r.desc }}</div>

        <div v-if="open[r.id]" style="margin-top: 10px; border-top: 1px solid var(--rule); padding-top: 10px">
          <div v-if="r.measures">
            <strong style="font-size: 13px">管控措施</strong>
            <div class="muted" style="margin-top: 4px; white-space: pre-wrap">{{ r.measures }}</div>
          </div>
          <div v-if="r.clauses" style="margin-top: 8px">
            <strong style="font-size: 13px">依据</strong>
            <div class="muted" style="margin-top: 4px">{{ r.clauses }}</div>
          </div>
          <div v-if="r.case" style="margin-top: 8px">
            <strong style="font-size: 13px">典型案例</strong>
            <div class="muted" style="margin-top: 4px; white-space: pre-wrap">{{ r.case }}</div>
          </div>
        </div>
        <div class="muted" style="margin-top: 8px; font-size: 12px">{{ open[r.id] ? '收起' : '展开措施与案例' }}</div>
      </div>
    </div>
  </div>
</template>
