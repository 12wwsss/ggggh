<script setup>
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { showToast } from 'vant'
import { api } from '../../shared/http'

const route = useRoute()
const questions = ref([])
const answers = ref({})
const result = ref(null)
const busy = ref(false)

function letterOf(option, index) {
  const m = /^([A-Za-z])[.、)．]/.exec(option || '')
  return m ? m[1].toUpperCase() : String.fromCharCode(65 + index)
}

onMounted(async () => {
  try {
    const qs = await api.get(`/api/v1/quiz/items/${route.params.id}/questions`)
    questions.value = qs.list || qs || []
  } catch (e) {
    showToast(e.message)
  }
})

async function submit() {
  if (Object.keys(answers.value).length < questions.value.length) {
    showToast('还有题目未作答')
    return
  }
  busy.value = true
  try {
    result.value = await api.post(`/api/v1/quiz/items/${route.params.id}/submit`, {
      answers: answers.value,
    })
    showToast(result.value.score >= 60 ? '通过' : '未通过，建议重新学习')
  } catch (e) {
    showToast(e.message)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="app-shell">
    <van-nav-bar title="岗位小测" left-arrow @click-left="$router.back()" />
    <div class="page pad">
      <div v-if="result" class="card">
        <div class="row-between">
          <strong>得分 {{ result.score }}</strong>
          <van-tag :type="result.passed ? 'success' : 'danger'">{{ result.passed ? '通过' : '未通过' }}</van-tag>
        </div>
        <div class="muted" style="margin-top: 6px">答对 {{ result.correct }} / {{ result.total }} 题</div>
      </div>

      <div v-for="(q, qi) in questions" :key="q.id" class="card">
        <div><strong>{{ qi + 1 }}. {{ q.stem }}</strong></div>
        <van-radio-group v-model="answers[q.id]" style="margin-top: 10px" :disabled="Boolean(result)">
          <van-cell-group inset>
            <van-cell
              v-for="(opt, oi) in q.options"
              :key="oi"
              clickable
              @click="!result && (answers[q.id] = letterOf(opt, oi))"
            >
              <template #title>
                <div class="row">
                  <van-radio :name="letterOf(opt, oi)" />
                  <span>{{ opt }}</span>
                </div>
              </template>
            </van-cell>
          </van-cell-group>
        </van-radio-group>

        <div v-if="result" style="margin-top: 10px">
          <div class="muted">
            正确答案：{{ result.questions[qi]?.answer }} · 你的选择：{{ result.questions[qi]?.your_answer || '未答' }}
          </div>
          <div v-if="result.questions[qi]?.analysis" class="muted" style="margin-top: 4px">
            解析：{{ result.questions[qi].analysis }}
          </div>
        </div>
      </div>

      <div v-if="!result && questions.length" style="margin: 16px 0">
        <van-button block type="primary" :loading="busy" @click="submit">提交答卷</van-button>
      </div>
      <van-empty v-if="!questions.length" description="该条目暂无小测题" />
    </div>
  </div>
</template>
