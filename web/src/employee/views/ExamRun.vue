<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showDialog, showToast } from 'vant'
import { api } from '../../shared/http'

const route = useRoute()
const router = useRouter()

const paper = ref(null)
const questions = ref([])
const answers = ref({})
const remain = ref(0)
const result = ref(null)
const busy = ref(false)
const started = ref(false)
let timer = null

const timeText = computed(() => {
  const s = Math.max(0, remain.value)
  const m = Math.floor(s / 60)
  return `${String(m).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
})

function letterOf(option, index) {
  const m = /^([A-Za-z])[.、)．]/.exec(option || '')
  return m ? m[1].toUpperCase() : String.fromCharCode(65 + index)
}

onMounted(async () => {
  try {
    const data = await api.post(`/api/v1/exam/papers/${route.params.id}/start`)
    paper.value = data.paper
    questions.value = data.questions || []
    remain.value = data.remaining_second || 0
    started.value = true
    const record = data.record
    if (data.submitted || (record && record.status !== 'in_progress')) {
      // 已交卷：展示成绩与答案，供复习
      result.value = {
        score: record.score,
        passed: record.passed,
        total_score: paper.value.total_score,
        answers: record.answers || {},
        questions: questions.value,
        review: true,
      }
      return
    }
    timer = setInterval(() => {
      remain.value -= 1
      if (remain.value <= 0) {
        clearInterval(timer)
        autoSubmit()
      }
    }, 1000)
  } catch (e) {
    showToast(e.message)
    router.back()
  }
})

onBeforeUnmount(() => timer && clearInterval(timer))

async function autoSubmit() {
  showToast('考试时间到，自动交卷')
  await submit(true)
}

async function submit(auto = false) {
  if (busy.value || result.value) return
  if (!auto) {
    const answered = Object.keys(answers.value).length
    if (answered < questions.value.length) {
      try {
        await showDialog({ title: '还有题目未作答', message: `已答 ${answered}/${questions.value.length}，确定交卷？` })
      } catch {
        return
      }
    }
  }
  busy.value = true
  try {
    const res = await api.post(`/api/v1/exam/papers/${route.params.id}/submit`, {
      answers: answers.value,
    })
    clearInterval(timer)
    showToast(res.passed ? '已通过' : '未通过')
    // 交卷后重新拉一次：拿含正确答案与解析的复习数据；失败也不影响成绩
    try {
      const data = await api.post(`/api/v1/exam/papers/${route.params.id}/start`)
      result.value = {
        ...res,
        answers: answers.value,
        questions: data.questions || questions.value,
        review: true,
      }
    } catch {
      result.value = { ...res, review: false }
    }
  } catch (e) {
    showToast(e.message)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="app-shell">
    <van-nav-bar :title="paper ? paper.title : '在线考试'" left-arrow @click-left="$router.back()" />
    <van-notice-bar
      v-if="started && !result"
      left-icon="clock-o"
      :text="`剩余时间 ${timeText}，时间到自动交卷`"
      color="#b45309"
      background="#fff7ed"
    />

    <div class="page pad">
      <div v-if="result" class="card">
        <div class="row-between">
          <strong style="font-size: 18px">{{ result.score }} 分</strong>
          <van-tag :type="result.passed ? 'success' : 'danger'">{{ result.passed ? '通过' : '未通过' }}</van-tag>
        </div>
        <div class="muted" style="margin-top: 6px">满分 {{ result.total_score }} 分，成绩已记录</div>
        <div style="margin-top: 12px">
          <van-button block @click="router.replace('/exams')">返回考试列表</van-button>
        </div>
      </div>

      <template v-if="result && result.review">
        <div class="muted" style="margin: 14px 0 4px">答案与解析</div>
        <div v-for="(q, qi) in result.questions" :key="q.id" class="card">
          <div><strong>{{ qi + 1 }}. {{ q.stem }}</strong></div>
          <div class="muted" style="margin-top: 4px">
            我的答案：{{ result.answers[q.id] || '未作答' }} · 正确答案：{{ q.answer }}
          </div>
          <div
            v-for="(opt, oi) in q.options"
            :key="oi"
            :style="{
              marginTop: '6px',
              color:
                letterOf(opt, oi) === q.answer
                  ? '#47624b'
                  : letterOf(opt, oi) === result.answers[q.id]
                    ? '#a5311f'
                    : '',
            }"
          >
            {{ opt }}
          </div>
          <div v-if="q.analysis" class="muted" style="margin-top: 6px">解析：{{ q.analysis }}</div>
        </div>
      </template>

      <template v-if="!result">
        <div v-for="(q, qi) in questions" :key="q.id" class="card">
          <div><strong>{{ qi + 1 }}. {{ q.stem }}</strong></div>
          <van-radio-group v-model="answers[q.id]" style="margin-top: 10px">
            <van-cell
              v-for="(opt, oi) in q.options"
              :key="oi"
              clickable
              @click="answers[q.id] = letterOf(opt, oi)"
            >
              <template #title>
                <div class="row">
                  <van-radio :name="letterOf(opt, oi)" />
                  <span>{{ opt }}</span>
                </div>
              </template>
            </van-cell>
          </van-radio-group>
        </div>
        <div v-if="questions.length" style="margin: 16px 0">
          <van-button block type="primary" :loading="busy" @click="submit(false)">交卷</van-button>
        </div>
      </template>
    </div>
  </div>
</template>
