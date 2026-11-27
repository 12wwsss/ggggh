<script setup>
import { onMounted, ref } from 'vue'
import { showDialog, showToast } from 'vant'
import { api } from '../../shared/http'

const items = ref([])
const loading = ref(true)
const letters = 'ABCDEF'

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/wrongbook')
    items.value = data.items || []
  } catch (e) {
    showToast(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function master(q) {
  try {
    await showDialog({ title: '移出错题本', message: '确认已经掌握这道题？移出后不再出现在这里。' })
  } catch {
    return
  }
  try {
    await api.post('/api/wrongbook', { id: q.id, ok: true })
    items.value = items.value.filter((x) => x.id !== q.id)
    showToast('已移出')
  } catch (e) {
    showToast(e.message)
  }
}
</script>

<template>
  <div class="page">
    <van-nav-bar title="错题本" left-arrow @click-left="$router.back()" />
    <div class="pad">
      <div class="muted" style="margin: 4px 0 10px">共 {{ items.length }} 道；答对后会自动移出</div>
      <van-empty v-if="!loading && !items.length" description="还没有错题" />
      <div v-for="q in items" :key="q.id" class="card">
        <div class="row-between">
          <strong style="line-height: 1.6">{{ q.q }}</strong>
          <van-tag plain>错 {{ q.times }} 次</van-tag>
        </div>
        <div v-for="(o, i) in q.o || []" :key="i" class="muted" style="margin-top: 6px">
          {{ letters[i] }}. {{ o }}
        </div>
        <div style="margin-top: 8px; color: var(--moss)">
          正确答案：{{ letters[q.a] }}. {{ (q.o || [])[q.a] || '' }}
        </div>
        <div v-if="q.e" class="muted" style="margin-top: 6px; white-space: pre-wrap">解析：{{ q.e }}</div>
        <div v-if="q.c" class="muted" style="margin-top: 4px">分类：{{ q.c }}<span v-if="q.job"> · {{ q.job }}</span></div>
        <div style="margin-top: 10px">
          <van-button size="small" plain @click="master(q)">已掌握，移出</van-button>
        </div>
      </div>
    </div>
  </div>
</template>
