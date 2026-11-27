<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showDialog, showToast } from 'vant'
import { api } from '../../shared/http'

const router = useRouter()
const items = ref([])
const unread = ref(0)
const onlyUnread = ref(false)
const loading = ref(true)

const kindText = { issue: '工单', broadcast: '宣贯', training: '带培', qa: '问答', report: '报表' }
const visible = computed(() => (onlyUnread.value ? items.value.filter((m) => !m.read) : items.value))

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/messages', onlyUnread.value ? { unread: 1 } : {})
    items.value = data.items || []
    unread.value = data.unread || 0
  } catch (e) {
    showToast(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function markRead(m) {
  if (m.read) return
  try {
    const res = await api.post('/api/messages/read', { id: m.id })
    m.read = true
    unread.value = res.unread || 0
  } catch (e) {
    showToast(e.message)
  }
}

async function open(m) {
  await markRead(m)
  if (m.ref_type === 'issue') return router.push('/issues')
  if (m.ref_type === 'broadcast') return router.push('/broadcasts')
  if (m.ref_type === 'training') return router.push('/training')
  if (m.ref_type === 'qa_ticket') {
    return showDialog({ title: m.title || '安全科回复', message: m.body || '' })
  }
  return showDialog({ title: m.title, message: m.body || '' })
}

async function readAll() {
  const todo = items.value.filter((m) => !m.read)
  for (const m of todo) {
    try {
      await api.post('/api/messages/read', { id: m.id })
      m.read = true
    } catch {
      break
    }
  }
  await load()
  showToast('已全部已读')
}
</script>

<template>
  <div class="page">
    <van-nav-bar title="消息中心" left-arrow @click-left="$router.back()">
      <template #right>
        <span v-if="unread" style="color: var(--seal)" @click="readAll">全部已读</span>
      </template>
    </van-nav-bar>

    <div class="pad">
      <div class="row-between" style="margin: 4px 0 10px">
        <span class="muted">未读 {{ unread }} 条</span>
        <van-switch v-model="onlyUnread" size="22" active-text="只看未读" @change="load" />
      </div>

      <van-empty v-if="!loading && !visible.length" description="暂时没有消息" />
      <div v-for="m in visible" :key="m.id" class="card" style="cursor: pointer" @click="open(m)">
        <div class="row-between">
          <strong :style="m.read ? '' : 'color: var(--seal)'">
            <span v-if="!m.read" style="margin-right: 4px">●</span>{{ m.title }}
          </strong>
          <van-tag plain>{{ kindText[m.kind] || m.kind }}</van-tag>
        </div>
        <div class="muted" style="margin-top: 6px; white-space: pre-wrap">{{ m.body }}</div>
        <div class="muted" style="margin-top: 6px; font-size: 12px">{{ m.created_at }}</div>
      </div>
    </div>
  </div>
</template>
