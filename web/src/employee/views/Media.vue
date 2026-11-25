<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../../shared/http'

const router = useRouter()
const items = ref([])
const loading = ref(false)
const cat = ref('全部')
const playing = ref('')
const playingTitle = ref('')

const cats = computed(() => {
  const set = new Set()
  items.value.forEach((x) => x.cat && set.add(x.cat))
  return ['全部', ...set]
})

const list = computed(() =>
  cat.value === '全部' ? items.value : items.value.filter((x) => x.cat === cat.value),
)

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/materials')
    items.value = data.items || []
  } catch (e) {
    // 静默失败，页面显示空
  } finally {
    loading.value = false
  }
}

function play(it) {
  if (it.type !== 'video') return
  playing.value = it.url
  playingTitle.value = it.title
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

onMounted(load)
</script>

<template>
  <div>
    <van-nav-bar title="视频资料" left-arrow @click-left="router.back()" />

    <div v-if="playing" class="player">
      <video :src="playing" controls autoplay playsinline style="width: 100%; background: #000" />
      <div class="player-bar">
        <span>{{ playingTitle }}</span>
        <span class="close" @click="playing = ''">关闭</span>
      </div>
    </div>

    <div class="pad">
      <div class="chips">
        <button
          v-for="c in cats"
          :key="c"
          :class="['chip', cat === c ? 'on' : '']"
          @click="(cat = c), (playing = '')"
        >
          {{ c }}
        </button>
      </div>

      <van-empty v-if="!loading && !list.length" description="暂时没有视频资料" />

      <div v-for="it in list" :key="it.id" class="media-item" @click="play(it)">
        <div class="ico">▶</div>
        <div class="grow">
          <div class="title">{{ it.title }}</div>
          <div class="muted">{{ it.cat }} · {{ it.at }}</div>
        </div>
        <div class="act">播放</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.pad {
  padding: 12px;
}
.player-bar {
  display: flex;
  justify-content: space-between;
  padding: 8px 12px;
  background: #111;
  color: #fff;
  font-size: 13px;
}
.close {
  color: #ffd400;
}
.chips {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 8px;
}
.chip {
  border: none;
  background: #fff;
  color: #59636e;
  padding: 6px 12px;
  border-radius: 16px;
  font-size: 13px;
  white-space: nowrap;
}
.chip.on {
  background: #a5311f;
  color: #fff;
}
.media-item {
  display: flex;
  align-items: center;
  gap: 10px;
  background: #fff;
  border-radius: 8px;
  padding: 12px;
  margin-bottom: 8px;
}
.ico {
  width: 38px;
  height: 38px;
  border-radius: 8px;
  background: #fff1e0;
  color: #a5311f;
  display: flex;
  align-items: center;
  justify-content: center;
}
.title {
  font-weight: 600;
  font-size: 14px;
}
.grow {
  flex: 1;
  min-width: 0;
}
.act {
  color: #a5311f;
  font-size: 13px;
}
.muted {
  color: #8a94a0;
  font-size: 12px;
  margin-top: 2px;
}
</style>
