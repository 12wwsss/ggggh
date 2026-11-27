<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import { api } from '../../shared/http'

const router = useRouter()
const items = ref([])
const total = ref(0)
const loading = ref(true)

const links = {
  rectify: '/issues',
  rejected: '/issues',
  accept: '/issues',
  verify: '/issues',
  close: '/issues',
  learn: '/learn',
  message: '/messages',
}

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/notifications')
    items.value = data.items || []
    total.value = data.total || 0
  } catch (e) {
    showToast(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)

function go(key) {
  router.push(links[key] || '/issues')
}
</script>

<template>
  <div class="page">
    <van-nav-bar title="待办" left-arrow @click-left="$router.back()" />
    <div class="pad">
      <div v-if="total" class="card" style="margin-bottom: 12px">
        <div class="muted">当前共有</div>
        <div style="font-size: 32px; font-family: var(--font-serif); color: var(--seal)">
          {{ total }} <span style="font-size: 14px">件待办</span>
        </div>
      </div>
      <van-empty v-if="!loading && !items.length" description="没有待办，保持住" />
      <div v-for="it in items" :key="it.key" class="card" style="cursor: pointer" @click="go(it.key)">
        <div class="row-between">
          <strong>{{ it.label }}</strong>
          <span style="font-size: 22px; color: var(--seal)">{{ it.count }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
