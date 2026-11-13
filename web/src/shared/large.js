import { ref, watch } from 'vue'

// 大字版开关：默认开启（目标用户是不会打字、看不清小字的矿工），
// 存在 localStorage 里，切换后立即作用于 html[data-large]。
const KEY = 'coalrag.large'
const stored = localStorage.getItem(KEY)
export const large = ref(stored === null ? true : stored === '1')

export function applyLarge() {
  document.documentElement.dataset.large = large.value ? '1' : '0'
}

watch(large, (v) => {
  localStorage.setItem(KEY, v ? '1' : '0')
  applyLarge()
})

applyLarge()

export function useLarge() {
  return large
}
