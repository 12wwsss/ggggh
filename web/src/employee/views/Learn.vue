<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../shared/http'

const router = useRouter()
const items = ref([])
const loading = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    const data = await api.get('/api/v1/learn/items', { page_size: 50 })
    items.value = data.list || []
  } catch (e) {
    console.warn(e.message)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page">
    <van-nav-bar title="岗位学习" />
    <div class="pad">
      <van-empty v-if="!loading && !items.length" description="暂无可学习条目" />
      <div v-for="it in items" :key="it.id" class="card" @click="router.push(`/learn/${it.id}`)">
        <div class="row-between">
          <strong>{{ it.title }}</strong>
          <van-tag plain type="primary">{{ it.category }}</van-tag>
        </div>
        <div class="muted" style="margin-top: 8px">
          <span v-if="it.post">适用岗位：{{ it.post }} · </span>
          <span v-if="it.clause_refs && it.clause_refs.length">
            关联条款：{{ it.clause_refs.join('、') }}
          </span>
          <span v-else>综合安全要求</span>
        </div>
        <div class="muted" style="margin-top: 8px">{{ it.content.slice(0, 60) }}…</div>
      </div>
    </div>
  </div>
</template>
