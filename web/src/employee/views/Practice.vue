<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import { api } from '../../shared/http'

const router = useRouter()
const letters = 'ABCDEF'
const stage = ref('start') // start | run | result
const jobs = ref([])
const job = ref('全部')
const counts = [5, 10, 20]
const count = ref(10)
const questions = ref([])
const answers = ref({})
const result = ref(null)
const loading = ref(false)

const answeredCount = computed(() => Object.keys(answers.value).length)

onMounted(async () => {
  try {
    const r = await api.get('/api/options')
    jobs.value = (r.jobs || []).filter(Boolean)
  } catch {
    jobs.value = []
  }
})

async function start() {
  loading.value = true
  try {
    const r = await api.get('/api/questions', { job: job.value === '全部' ? '' : job.value, count: count.value })
    const items = (r.items || []).map((q) => ({ ...q, options: q.o || q.options || [] }))
    if (!items.length) {
      showToast('这个岗位还没有题目')
      return
    }
    questions.value = items
    answers.value = {}
    result.value = null
    stage.value = 'run'
  } catch (e) {
    showToast(e.message)
  } finally {
    loading.value = false
  }
}

function choose(q, i) {
  answers.value = { ...answers.value, [q.id]: i }
}

async function submit() {
  if (answeredCount.value < questions.value.length) return showToast('还有题目未作答')
  loading.value = true
  try {
    let correct = 0
    const wrong = []
    for (const q of questions.value) {
      const ok = Number(answers.value[q.id]) === Number(q.a)
      if (ok) correct++
      else wrong.push({ id: q.id, q: q.q, a: q.a })
      api.post('/api/wrongbook', { id: q.id, ok }).catch(() => {})
    }
    const score = Math.round((correct / questions.value.length) * 100)
    result.value = { correct, total: questions.value.length, score }
    stage.value = 'result'
    api
      .post('/api/quiz', { score, total: questions.value.length, job: job.value, wrong })
      .catch(() => {})
  } catch (e) {
    showToast(e.message)
  } finally {
    loading.value = false
  }
}

function restart() {
  stage.value = 'start'
  questions.value = []
  answers.value = {}
  result.value = null
}
</script>

<template>
  <div class="page">
    <van-nav-bar title="练题" left-arrow @click-left="$router.back()" />

    <div class="pad">
      <template v-if="stage === 'start'">
        <div class="card">
          <strong>按岗位出题，答错自动进错题本</strong>
          <div class="muted" style="margin-top: 6px">题目全部来自题库，答完可以看解析</div>
        </div>

        <div class="muted" style="margin: 14px 0 6px">选择范围</div>
        <div class="lt-chips">
          <button
            v-for="j in ['全部', ...jobs]"
            :key="j"
            class="lt-chip"
            :class="{ on: job === j }"
            @click="job = j"
          >
            {{ j }}
          </button>
        </div>

        <div class="muted" style="margin: 14px 0 6px">题目数量</div>
        <div class="lt-chips lt-chips-count">
          <button
            v-for="c in counts"
            :key="c"
            class="lt-chip"
            :class="{ on: count === c }"
            @click="count = c"
          >
            {{ c }} 题
          </button>
        </div>

        <van-button block type="primary" style="margin-top: 20px" :loading="loading" @click="start">
          开始练题
        </van-button>
      </template>

      <template v-else-if="stage === 'run'">
        <div class="muted" style="margin: 4px 0 10px">已答 {{ answeredCount }} / {{ questions.length }}</div>
        <div v-for="(q, qi) in questions" :key="q.id" class="card">
          <strong>{{ qi + 1 }}. {{ q.q }}</strong>
          <div
            v-for="(opt, oi) in q.options"
            :key="oi"
            class="card"
            :style="{
              marginTop: '8px',
              cursor: 'pointer',
              background: answers[q.id] === oi ? 'var(--seal-tint)' : 'var(--sheet)',
            }"
            @click="choose(q, oi)"
          >
            {{ letters[oi] }}. {{ opt }}
          </div>
        </div>
        <van-button block type="primary" :loading="loading" @click="submit">提交</van-button>
      </template>

      <template v-else-if="stage === 'result' && result">
        <div class="card">
          <div class="row-between">
            <strong style="font-size: 18px">{{ result.score }} 分</strong>
            <van-tag :type="result.score >= 60 ? 'success' : 'danger'">
              答对 {{ result.correct }} / {{ result.total }}
            </van-tag>
          </div>
          <div class="muted" style="margin-top: 6px">成绩已记录，错题已进错题本</div>
        </div>

        <div class="muted" style="margin: 14px 0 4px">答案与解析</div>
        <div v-for="(q, qi) in questions" :key="q.id" class="card">
          <strong>{{ qi + 1 }}. {{ q.q }}</strong>
          <div class="muted" style="margin-top: 4px">
            我的答案：{{ letters[answers[q.id]] || '未答' }} · 正确答案：{{ letters[q.a] }}
          </div>
          <div
            v-for="(opt, oi) in q.options"
            :key="oi"
            :style="{
              marginTop: '6px',
              color:
                oi === Number(q.a)
                  ? 'var(--moss)'
                  : Number(answers[q.id]) === oi
                    ? 'var(--seal)'
                    : '',
            }"
          >
            {{ letters[oi] }}. {{ opt }}
          </div>
          <div v-if="q.e" class="muted" style="margin-top: 6px">解析：{{ q.e }}</div>
        </div>

        <div class="row" style="gap: 8px; margin-top: 8px">
          <van-button block type="primary" @click="restart">再练一组</van-button>
          <van-button block @click="router.push('/wrong')">看错题本</van-button>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.lt-chips {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}
.lt-chips-count {
  grid-template-columns: repeat(3, 1fr);
}
.lt-chip {
  min-height: 42px;
  padding: 6px 8px;
  border: 1px solid var(--rule-strong);
  border-radius: 2px;
  background: var(--paper-2);
  color: var(--ink);
  font-size: 14px;
  cursor: pointer;
}
.lt-chip.on {
  border-color: var(--seal);
  color: var(--seal);
  background: var(--seal-tint);
  font-weight: 700;
}
</style>
