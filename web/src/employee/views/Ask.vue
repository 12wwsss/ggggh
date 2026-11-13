<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, fetchText } from '../../shared/http'
import { showToast } from 'vant'

const router = useRouter()
const route = useRoute()

// 底栏 tab 用 replace 切换，从首页进 /ask 时历史里没有上一条记录，
// back() 无处可退；没有上一页就回首页，保证返回按钮始终有效。
function goBack() {
  if (window.history.state?.back) router.back()
  else router.replace('/')
}

const question = ref('')
const sending = ref(false)
const messages = ref([
  {
    role: 'ai',
    text: '我是安全员助手。问我规程条款、作业要求、隐患处置，我会给出依据条款；没有依据时会明确说明。',
    citations: [],
  },
])
const scroller = ref(null)
const showClause = ref(false)
const clauseText = ref('')
const clauseTitle = ref('')

const presets = [
  { label: '瓦斯超限怎么办', q: '瓦斯浓度达到1.5%时怎么办？' },
  { label: '停风了怎么办', q: '停风时怎么处理？' },
  { label: '透水征兆', q: '出现透水征兆怎么办？' },
  { label: '敲帮问顶', q: '敲帮问顶应该怎么做？' },
]


// 条款问答常见写法是“根据《…》第X条规定，按以下顺序执行：”再列步骤。
// 大字版用户要的是“先做什么”，所以把这句引子挪到依据块，列表提到最前；
// 问题里带数字（1.5%、0.8 米）时，把命中数字的那条提到第一条并标“先做”。
function leadText(m) {
  if (m.role !== 'ai') return ''
  return (m.text || '')
    .replace(/^\s*根据《[^》]*》[^：:]*[：:]\s*/u, '')
    .replace(/^\s*依据[:：].*$/gm, '')
    .trim()
}

function orderedActions(m) {
  const acts = (m.actions || []).slice()
  if (!acts.length) return []
  const nums = (m.question || '').match(/\d+(?:\.\d+)?%?/g) || []
  if (!nums.length) return acts.map((text) => ({ text, first: false }))
  const idx = acts.findIndex((a) => nums.some((n) => a.includes(n)))
  if (idx <= 0) return acts.map((text) => ({ text, first: idx === 0 }))
  const [hit] = acts.splice(idx, 1)
  return [{ text: hit, first: true }, ...acts.map((text) => ({ text, first: false }))]
}

async function scrollBottom() {
  await nextTick()
  if (scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight
}

async function send(text) {
  const q = (text ?? question.value).trim()
  if (!q || sending.value) return
  messages.value.push({ role: 'me', text: q })
  const prev = messages.value[messages.value.length - 1]
  question.value = ''
  sending.value = true
  await scrollBottom()
  try {
    const res = await api.post('/api/v1/qa/ask', { question: q })
    messages.value.push({
      role: 'ai',
      text: res.answer,
      question: q,
      actions: res.actions || [],
      citations: res.citations || [],
      answered: res.answered,
      logId: res.log_id,
      latency: res.latency_ms,
      feedback: '',
    })
  } catch (e) {
    messages.value.push({ role: 'ai', text: `出错了：${e.message}`, citations: [] })
  } finally {
    sending.value = false
    scrollBottom()
  }
}

// 回答里的“依据：[第X条]”整行由下方“依据”块统一承担，
// 避免同一条款出现两次，也让依据更像盖章位置。
function displayText(m) {
  if (m.role !== 'ai') return m.text
  return m.text
    .replace(/^\s*依据[:：].*$/gm, '')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

async function openClause(cite) {
  clauseTitle.value = `${cite.doc_title} ${cite.clause_no}`
  clauseText.value = '加载中…'
  showClause.value = true
  try {
    clauseText.value = await fetchText(`/api/v1/chunks/${cite.chunk_id}/text`)
  } catch (e) {
    clauseText.value = `原文加载失败：${e.message}`
  }
}

// 大字版用户多数不识字或看不清，优先提供“读给我听”。
// 浏览器不支持语音合成时明确告知，而不是静默失败。
function speak(m) {
  const text = displayText(m).replace(/\n+/g, '。')
  if (!text) return
  if (!('speechSynthesis' in window)) {
    showToast('这台设备不支持朗读')
    return
  }
  window.speechSynthesis.cancel()
  const u = new SpeechSynthesisUtterance(text)
  u.lang = 'zh-CN'
  u.rate = 0.9
  window.speechSynthesis.speak(u)
}

function speakPreset() {
  if (!('speechSynthesis' in window)) {
    showToast('这台设备不支持朗读')
    return
  }
  window.speechSynthesis.cancel()
}

function voiceInput() {
  const Rec = window.SpeechRecognition || window.webkitSpeechRecognition
  if (!Rec) {
    showToast('这台设备不支持语音输入，可以点下面的常问')
    return
  }
  const rec = new Rec()
  rec.lang = 'zh-CN'
  rec.interimResults = false
  rec.onresult = (e) => {
    const text = e.results?.[0]?.[0]?.transcript || ''
    if (text) {
      question.value = text
      send(text)
    }
  }
  rec.onerror = () => showToast('没听清，再说一次或点下面的常问')
  rec.start()
  showToast('请说话…')
}

async function feedback(msg, value) {
  if (!msg.logId || msg.feedback) return
  try {
    await api.post(`/api/v1/qa/logs/${msg.logId}/feedback`, { feedback: value })
    msg.feedback = value
    showToast('感谢反馈')
  } catch (e) {
    showToast(e.message)
  }
}

onMounted(() => {
  const q = typeof route.query.q === 'string' ? route.query.q : ''
  if (q) send(q)
})

onBeforeUnmount(() => {
  if ('speechSynthesis' in window) window.speechSynthesis.cancel()
  speakPreset()
})
</script>

<template>
  <div class="app-shell">
    <van-nav-bar title="问规程" left-arrow @click-left="goBack" />

    <div ref="scroller" class="page pad">
      <div v-for="(m, i) in messages" :key="i" :class="['bubble', m.role === 'me' ? 'me' : 'ai']">
        <div v-if="m.role === 'me'">{{ displayText(m) }}</div>
        <ol v-if="m.role === 'ai' && m.actions && m.actions.length" class="actions">
          <li v-for="(a, ai) in orderedActions(m)" :key="ai">
            <span v-if="a.first" class="lt-first">先做</span>{{ a.text }}
          </li>
        </ol>
        <div v-if="m.role === 'ai' && leadText(m)">{{ leadText(m) }}</div>
        <div v-if="m.role === 'ai' && m.citations && m.citations.length" class="cite-block">
          <span class="cite-label">依据</span>
          <span v-for="c in m.citations" :key="c.chunk_id" class="cite" @click="openClause(c)">
            {{ c.clause_no }}
          </span>
        </div>
        <div v-if="m.role === 'ai' && m.text && m !== messages[0]" class="lt-answer-acts">
          <button class="lt-mini" @click="speak(m)">读给我听</button>
          <button
            v-if="m.citations && m.citations.length"
            class="lt-mini"
            @click="openClause(m.citations[0])"
          >
            再看原文
          </button>
        </div>
        <div v-if="m.role === 'ai' && m.logId" class="row" style="margin-top: 8px">
          <span class="muted num">{{ m.latency >= 100 ? m.latency + 'ms' : '' }}</span>
          <span class="muted" style="margin-left: auto">
            <a v-if="!m.feedback" @click="feedback(m, 'useful')">有用</a>
            <a v-if="!m.feedback" style="margin-left: 12px" @click="feedback(m, 'useless')">没用</a>
            <span v-if="m.feedback">已反馈</span>
          </span>
        </div>
      </div>

      <div v-if="!messages.some((m) => m.role === 'me')" class="lt-presets">
        <div class="lt-sec">常问的（点一下就行）</div>
        <div class="lt-chips">
          <button v-for="p in presets" :key="p.label" class="lt-chip" @click="send(p.q)">
            {{ p.label }}
          </button>
        </div>
      </div>
    </div>

    <div class="composer lt-composer">
      <button class="lt-mic" @click="voiceInput">说</button>
      <input
        v-model="question"
        class="lt-input"
        placeholder="打字，或按左边说话"
        enterkeyhint="send"
        @keyup.enter="send()"
      />
      <button class="lt-send" :disabled="sending" @click="send()">
        {{ sending ? '…' : '发送' }}
      </button>
    </div>

    <van-popup v-model:show="showClause" position="bottom" round :style="{ maxHeight: '75%' }">
      <div class="pad">
        <div class="row-between">
          <strong>{{ clauseTitle }}</strong>
          <span class="muted" @click="showClause = false">关闭</span>
        </div>
        <div style="margin-top: 12px; white-space: pre-wrap; line-height: 1.8">{{ clauseText }}</div>
      </div>
    </van-popup>
  </div>
</template>

<style scoped>
.lt-answer-acts {
  display: flex;
  gap: 10px;
  margin-top: 12px;
}
.lt-mini {
  min-height: 44px;
  padding: 6px 14px;
  border: 2px solid var(--rule-strong);
  border-radius: 4px;
  background: var(--sheet);
  color: var(--ink);
  font-size: 17px;
}
.lt-mini:active {
  border-color: var(--seal);
  color: var(--seal);
}

.lt-first {
  display: inline-block;
  white-space: nowrap;
  margin-right: 8px;
  padding: 2px 8px;
  background: var(--seal);
  color: #fbf6ec;
  font-size: 15px;
  letter-spacing: 0.1em;
  vertical-align: 2px;
}
.lt-presets {
  margin-top: 4px;
}
.lt-sec {
  font-size: 17px;
  color: var(--ink-2);
  letter-spacing: 0.08em;
  margin-bottom: 12px;
}
.lt-chips {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.lt-chip {
  min-height: 60px;
  padding: 12px;
  border: 2px solid var(--rule-strong);
  border-radius: 4px;
  background: var(--sheet);
  color: var(--ink);
  font-size: 19px;
  line-height: 1.4;
}
.lt-chip:active {
  border-color: var(--seal);
  color: var(--seal);
}

.lt-composer {
  align-items: center;
  gap: 10px;
  padding: 12px 14px calc(12px + env(safe-area-inset-bottom));
}
.lt-mic {
  flex: 0 0 56px;
  height: 56px;
  border: 2px solid var(--seal);
  border-radius: 4px;
  background: var(--seal);
  color: #fbf6ec;
  font-size: 20px;
  font-weight: 700;
}
.lt-input {
  flex: 1;
  min-width: 0;
  height: 56px;
  padding: 0 14px;
  border: 2px solid var(--ink);
  border-radius: 4px;
  background: #fff;
  font-size: 19px;
  color: var(--ink);
}
.lt-send {
  flex: 0 0 auto;
  min-width: 68px;
  height: 56px;
  padding: 0 10px;
  border: 2px solid var(--rule-strong);
  border-radius: 4px;
  background: var(--sheet);
  color: var(--ink-2);
  font-size: 18px;
}
</style>
