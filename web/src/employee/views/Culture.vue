<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import { api } from '../../shared/http'

const router = useRouter()
const loading = ref(true)
const items = ref([])
const cat = ref('')

// 分类同时覆盖 article（案例/安全月/知识卡）与 culture（寄语/光荣榜）两类内容。
const CATS = ['全部', '案例', '安全月', '寄语', '光荣榜', '知识卡']

const shown = computed(() => (cat.value ? items.value.filter((it) => it.cat === cat.value) : items.value))

function go(item) {
  router.push(`/culture/${item.id}`)
}

onMounted(async () => {
  try {
    const [articles, cultures] = await Promise.all([
      api.get('/api/v1/culture', { kind: 'article' }),
      api.get('/api/v1/culture', { kind: 'culture' }),
    ])
    items.value = [...((articles?.items) || []), ...((cultures?.items) || [])].map((it) => ({
      ...it,
      summary: it.summary || (Array.isArray(it.body) ? it.body[0] : it.body) || '',
    }))
  } catch (e) {
    showToast(e.message)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page">
    <van-nav-bar title="安全文化" left-arrow @click-left="$router.back()" />
    <div class="pad">
      <div class="lt-chips" style="grid-template-columns: repeat(3, 1fr); gap: 8px">
        <button
          v-for="c in CATS"
          :key="c"
          class="lt-chip"
          :class="{ 'is-on': (c === '全部' && !cat) || cat === c }"
          @click="cat = c === '全部' ? '' : c"
        >
          {{ c }}
        </button>
      </div>

      <van-empty v-if="!loading && !shown.length" description="暂时还没有内容" />

      <div v-for="it in shown" :key="it.id" class="card" style="margin-top: 12px" @click="go(it)">
        <div class="row-between">
          <strong style="font-size: 17px; flex: 1; min-width: 0; margin-right: 10px">{{ it.title }}</strong>
          <van-tag plain type="danger" style="flex: 0 0 auto; white-space: nowrap">{{ it.tag || it.cat }}</van-tag>
        </div>
        <div class="muted" style="margin-top: 8px; line-height: 1.7">
          {{ it.summary }}
        </div>
        <div class="muted" style="margin-top: 10px; font-size: 13px">
          {{ it.date || '' }}
          <span v-if="it.minutes"> · 阅读约 {{ it.minutes }} 分钟</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.lt-chip {
  min-height: 48px;
  font-size: 16px;
}
.lt-chip.is-on {
  border-color: var(--seal);
  color: var(--seal);
  background: var(--seal-tint);
}
</style>
