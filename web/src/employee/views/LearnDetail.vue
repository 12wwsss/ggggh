<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast } from 'vant'
import { api, fetchText } from '../../shared/http'

const route = useRoute()
const router = useRouter()
const item = ref(null)
const record = ref(null)
const questions = ref([])
const busy = ref(false)
const startedAt = Date.now()
const showClause = ref(false)
const clauseText = ref('')

const completed = computed(() => record.value?.status === 'completed')

onMounted(load)

async function load() {
  try {
    const data = await api.get(`/api/v1/learn/items/${route.params.id}`)
    item.value = data.item
    record.value = data.record
    const qs = await api.get(`/api/v1/quiz/items/${route.params.id}/questions`)
    questions.value = qs.list || qs || []
  } catch (e) {
    showToast(e.message)
  }
}

async function finish() {
  busy.value = true
  try {
    const seconds = Math.max(5, Math.round((Date.now() - startedAt) / 1000))
    record.value = await api.post(`/api/v1/learn/items/${route.params.id}/progress`, {
      progress: 100,
      duration_sec: seconds,
    })
    showToast('已标记为完成')
  } catch (e) {
    showToast(e.message)
  } finally {
    busy.value = false
  }
}

// 关联条款可反查原文（按条款号在文档里检索）
async function openClause(ref) {
  clauseText.value = '加载中…'
  showClause.value = true
  try {
    const hits = await api.post('/api/v1/qa/search', { question: ref, top_k: 1, min_score: 0 })
    const first = (hits.hits || [])[0]
    clauseText.value = first ? await fetchText(`/api/v1/chunks/${first.chunk_id}/text`) : '未找到该条款原文'
  } catch (e) {
    clauseText.value = e.message
  }
}
</script>

<template>
  <div class="app-shell">
    <van-nav-bar title="学习详情" left-arrow @click-left="$router.back()" />
    <div class="page pad">
      <div v-if="item" class="card">
        <div class="row-between">
          <strong style="font-size: 16px">{{ item.title }}</strong>
          <van-tag plain type="primary">{{ item.category }}</van-tag>
        </div>
        <div class="muted" style="margin-top: 6px">
          <span v-if="item.post">适用岗位：{{ item.post }}</span>
        </div>
        <div style="margin-top: 12px; line-height: 1.8; white-space: pre-wrap">{{ item.content }}</div>

        <div v-if="item.clause_refs && item.clause_refs.length" style="margin-top: 12px">
          <div class="muted">关联条款</div>
          <van-tag
            v-for="ref in item.clause_refs"
            :key="ref"
            type="primary"
            plain
            style="margin: 6px 8px 0 0"
            @click="openClause(ref)"
            >{{ ref }}</van-tag
          >
        </div>
      </div>

      <div class="card">
        <div class="row-between">
          <strong>学习进度</strong>
          <span class="muted">{{ completed ? '已完成' : record?.progress ? `已学 ${record.progress}%` : '未开始' }}</span>
        </div>
        <van-progress
          :percentage="record?.progress || 0"
          stroke-width="6"
          color="#a5311f"
          style="margin-top: 10px"
        />
        <div style="margin-top: 14px; display: flex; gap: 10px">
          <van-button :disabled="completed" :loading="busy" type="primary" block @click="finish">
            {{ completed ? '已完成' : '标记完成' }}
          </van-button>
          <van-button
            v-if="questions.length"
            :disabled="!completed"
            plain
            type="primary"
            block
            @click="router.push(`/quiz/${item.id}`)"
          >
            去小测（{{ questions.length }} 题）
          </van-button>
        </div>
        <div v-if="questions.length && !completed" class="muted" style="margin-top: 8px">
          完成学习后可参加小测
        </div>
      </div>
    </div>

    <van-popup v-model:show="showClause" position="bottom" round :style="{ maxHeight: '75%' }">
      <div class="pad">
        <div class="row-between">
          <strong>条款原文</strong>
          <span class="muted" @click="showClause = false">关闭</span>
        </div>
        <div style="margin-top: 12px; white-space: pre-wrap; line-height: 1.8">{{ clauseText }}</div>
      </div>
    </van-popup>
  </div>
</template>
