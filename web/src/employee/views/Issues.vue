<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../shared/http'
import { fmtTime, issueStatus, severityText } from '../../shared/format'

const router = useRouter()
const list = ref([])
const loading = ref(true)

onMounted(async () => {
  try {
    const data = await api.get('/api/v1/issues', { page_size: 50 })
    list.value = data.list || []
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page">
    <van-nav-bar title="我的问题" left-arrow @click-left="$router.back()">
      <template #right>
        <span style="color: var(--seal)" @click="router.push('/issues/new')">上报</span>
      </template>
    </van-nav-bar>
    <div class="pad">
      <van-empty v-if="!loading && !list.length" description="还没有上报记录" />
      <div v-for="i in list" :key="i.id" class="card" @click="router.push(`/issues/${i.id}`)">
        <div class="row-between">
          <strong>{{ i.title }}</strong>
          <van-tag :type="issueStatus[i.status]?.type || 'default'">{{ issueStatus[i.status]?.text || i.status }}</van-tag>
        </div>
        <div class="muted" style="margin-top: 8px">
          {{ i.code }} · {{ i.cate_name || '未分类' }} · {{ severityText[i.severity] }}
          <span v-if="i.overdue" style="color: #d93025"> · 已超时</span>
        </div>
        <div class="muted" style="margin-top: 6px">
          {{ i.dept_name || '—' }} · {{ fmtTime(i.created_at) }}
        </div>
      </div>
    </div>
  </div>
</template>
