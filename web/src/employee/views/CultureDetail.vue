<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast } from 'vant'
import { api } from '../../shared/http'

const route = useRoute()
const router = useRouter()
const item = ref(null)
const loading = ref(true)
const err = ref('')

const paras = computed(() => {
  const b = item.value?.body
  if (Array.isArray(b)) return b.filter(Boolean)
  if (typeof b === 'string') return b.split('\n').map((s) => s.trim()).filter(Boolean)
  return []
})

function askAI() {
  const q = item.value?.ask || item.value?.title || ''
  router.push({ path: '/ask', query: q ? { q } : {} })
}

onMounted(async () => {
  try {
    const data = await api.get(`/api/v1/culture/${route.params.id}`)
    item.value = data.item
  } catch (e) {
    err.value = e.message
    showToast(e.message)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page">
    <van-nav-bar :title="item?.cat || '内容'" left-arrow @click-left="$router.back()" />
    <div class="pad">
      <van-empty v-if="err" :description="err" />
      <template v-else-if="item">
        <div class="card">
          <div class="row-between">
            <strong style="font-size: 20px">{{ item.title }}</strong>
            <van-tag plain type="danger">{{ item.tag || item.cat }}</van-tag>
          </div>
          <div class="muted" style="margin-top: 8px; font-size: 13px">
            {{ item.date || '' }}
            <span v-if="item.minutes"> · 阅读约 {{ item.minutes }} 分钟</span>
          </div>
          <div v-if="item.summary" class="muted" style="margin-top: 12px; line-height: 1.8">
            {{ item.summary }}
          </div>
          <div v-for="(p, i) in paras" :key="i" style="margin-top: 14px; line-height: 1.9; font-size: 17px">
            {{ p }}
          </div>
        </div>

        <div class="card" style="margin-top: 12px">
          <div class="muted" style="font-size: 13px">内容仅供参考，以规程原文和现场指令为准</div>
          <van-button block type="primary" style="margin-top: 12px" @click="askAI">问问 AI</van-button>
        </div>
      </template>
      <div v-else-if="loading" class="muted pad">加载中…</div>
    </div>
  </div>
</template>
