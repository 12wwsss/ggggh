<script setup>
import { computed, onMounted, ref } from 'vue'
import { showToast } from 'vant'
import { api } from '../../shared/http'

const plans = ref([])
const active = ref(0)
const loading = ref(true)
const busy = ref(false)
const form = ref({ kind: 'monthly', period: new Date().toISOString().slice(0, 7), content: '' })

const labels = computed(() =>
  plans.value.map((p) => (p.asMentor ? `${p.userName || '学员'} · ${p.title}` : p.title)),
)
const plan = computed(() => plans.value[active.value] || null)
const progress = computed(() => {
  const p = plan.value
  if (!p || !p.stages || !p.stages.length) return 0
  return Math.round((p.stages.filter((s) => s.done).length / p.stages.length) * 100)
})

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/training')
    plans.value = data.plans || []
  } catch (e) {
    showToast(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function toggle(stage) {
  if (!plan.value || busy.value) return
  busy.value = true
  try {
    await api.post('/api/training/stage', { stage_id: stage.id, done: !stage.done })
    showToast(stage.done ? '已撤销' : '已确认完成')
    await load()
  } catch (e) {
    showToast(e.message)
  } finally {
    busy.value = false
  }
}

async function saveNote() {
  if (!plan.value) return
  const content = form.value.content.trim()
  if (content.length < 10) return showToast('小结至少写 10 个字')
  if (!/^\d{4}-\d{2}$/.test(form.value.period)) return showToast('月份格式如 2026-11')
  busy.value = true
  try {
    await api.post('/api/training/note', {
      plan_id: plan.value.id,
      kind: form.value.kind,
      period: form.value.period,
      content,
    })
    form.value.content = ''
    showToast('小结已提交')
    await load()
  } catch (e) {
    showToast(e.message)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="page">
    <van-nav-bar title="新员工带培" left-arrow @click-left="$router.back()" />
    <div class="pad">
      <van-empty v-if="!loading && !plans.length" description="还没有带培计划，请联系培训员或师傅" />

      <template v-if="plan">
        <van-tabs v-if="plans.length > 1" v-model:active="active" style="margin-bottom: 12px">
          <van-tab v-for="(l, i) in labels" :key="i" :title="l" />
        </van-tabs>

        <div class="card">
          <div class="row-between">
            <strong>{{ plan.title }}</strong>
            <van-tag>{{ progress }}%</van-tag>
          </div>
          <div class="muted" style="margin-top: 6px">
            <span v-if="plan.asMentor">学员：{{ plan.userName }} · </span>共 {{ plan.totalWeeks }} 周 · 师傅
            {{ plan.mentor }}
          </div>
          <div v-if="plan.target" class="muted" style="margin-top: 4px">目标：{{ plan.target }}</div>
          <van-progress :percentage="progress" :show-pivot="false" style="margin-top: 10px" />
        </div>

        <div class="muted" style="margin: 14px 0 6px">阶段进度</div>
        <div v-for="(st, i) in plan.stages" :key="st.id" class="card">
          <div class="row-between">
            <strong>{{ i + 1 }}. {{ st.goal }}</strong>
            <van-tag :type="st.done ? 'success' : 'default'">{{ st.done ? '已完成' : st.name }}</van-tag>
          </div>
          <div class="muted" style="margin-top: 6px">{{ (st.tasks || []).join('；') }}</div>
          <div v-if="plan.canWrite" style="margin-top: 10px">
            <van-button size="small" :type="st.done ? 'default' : 'primary'" :loading="busy" @click="toggle(st)">
              {{ st.done ? '撤销完成' : '确认完成' }}
            </van-button>
          </div>
        </div>

        <div class="muted" style="margin: 14px 0 6px">师傅小结</div>
        <div v-if="plan.canWrite" class="card">
          <div class="row" style="gap: 8px">
            <van-field
              v-model="form.period"
              label="月份"
              placeholder="2026-11"
              style="flex: 1; padding-left: 0"
            />
            <van-radio-group v-model="form.kind" direction="horizontal">
              <van-radio name="monthly">月度</van-radio>
              <van-radio name="final">期满</van-radio>
            </van-radio-group>
          </div>
          <van-field
            v-model="form.content"
            type="textarea"
            rows="3"
            autosize
            placeholder="本月跟班情况、掌握的技能、存在的问题与下月重点（至少 10 字）"
          />
          <div style="margin-top: 10px">
            <van-button size="small" type="primary" :loading="busy" @click="saveNote">提交小结</van-button>
          </div>
        </div>

        <van-empty v-if="!(plan.notes || []).length" description="还没有小结" :image-size="60" />
        <div v-for="n in plan.notes" :key="n.id" class="card">
          <div class="row-between">
            <strong>{{ n.period }} · {{ n.kind === 'final' ? '期满总结' : '月度小结' }}</strong>
            <span class="muted">{{ n.at }}</span>
          </div>
          <div class="muted" style="margin-top: 4px">{{ n.author }}</div>
          <div style="margin-top: 8px; white-space: pre-wrap">{{ n.content }}</div>
        </div>
      </template>
    </div>
  </div>
</template>
