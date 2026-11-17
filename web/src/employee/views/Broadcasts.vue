<script setup>
import { onMounted, ref } from 'vue'
import { showToast } from 'vant'
import { api } from '../../shared/http'
import { fmtDate } from '../../shared/format'

const list = ref([])
const loading = ref(true)
const busy = ref(0)

async function load() {
  loading.value = true
  try {
    list.value = (await api.get('/api/v1/broadcast/mine', { pending: false })) || []
  } catch (e) {
    showToast(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function ack(task) {
  busy.value = task.id
  try {
    await api.post(`/api/v1/broadcast/tasks/${task.id}/ack`, { ack: true })
    showToast('已签收')
    await load()
  } catch (e) {
    showToast(e.message)
  } finally {
    busy.value = 0
  }
}

function deadLineText(task) {
  if (!task.deadline) return '无截止时间'
  const d = new Date(task.deadline)
  return `${fmtDate(task.deadline, true)}${d.getTime() < Date.now() ? '（已逾期）' : ''}`
}
</script>

<template>
  <div class="page">
    <van-nav-bar title="宣贯签收" left-arrow @click-left="$router.back()" />
    <div class="pad">
      <van-empty v-if="!loading && !list.length" description="暂无待签收的宣贯" />
      <div v-for="t in list" :key="t.id" class="card">
        <div class="row-between">
          <strong>{{ t.title }}</strong>
          <van-tag v-if="t.my_status === 'ack'" type="success">已签收</van-tag>
          <van-tag v-else :type="t.require_ack ? 'warning' : 'primary'">{{ t.require_ack ? '需签收' : '知悉' }}</van-tag>
        </div>
        <div class="muted" style="margin-top: 8px">{{ deadLineText(t) }}</div>
        <div style="margin-top: 10px; line-height: 1.8; white-space: pre-wrap">{{ t.content }}</div>
        <div v-if="t.my_status !== 'ack'" style="margin-top: 12px">
          <van-button block type="primary" size="small" :loading="busy === t.id" @click="ack(t)">
            确认签收
          </van-button>
        </div>
      </div>
    </div>
  </div>
</template>
