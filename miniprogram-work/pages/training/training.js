const api = require('../../utils/api.js')

function nowPeriod() {
  const d = new Date()
  const m = d.getMonth() + 1
  return d.getFullYear() + '-' + (m < 10 ? '0' + m : m)
}

Page({
  data: {
    plan: null,
    progress: 0,
    notes: [],
    canWrite: false,
    kinds: ['月度小结', '期满总结'],
    kindIndex: 0,
    period: '',
    content: '',
    saving: false,
    loading: true,
    err: ''
  },

  onLoad() {
    this.setData({ period: nowPeriod() })
    this.fetch()
  },

  fetch() {
    this.setData({ loading: true, err: '' })
    api.training().then(res => {
      const plan = (res.plans || [])[0] || null
      let progress = 0
      let notes = []
      if (plan) {
        const stages = plan.stages || []
        const done = stages.filter(s => s.done).length
        progress = stages.length ? Math.round((done / stages.length) * 100) : 0
        notes = (plan.notes || []).map(n => ({
          id: n.id,
          kind: n.kind,
          period: n.period,
          content: n.content,
          author: n.author || '—',
          at: n.at || ''
        }))
      }
      this.setData({
        plan,
        progress,
        notes,
        canWrite: !!(plan && plan.canWrite),
        loading: false
      })
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
    })
  },

  onKindChange(e) {
    this.setData({ kindIndex: Number(e.detail.value) || 0 })
  },

  onPeriod(e) {
    this.setData({ period: e.detail.value })
  },

  onContent(e) {
    this.setData({ content: e.detail.value })
  },

  saveNote() {
    const plan = this.data.plan
    if (!plan) return
    const content = (this.data.content || '').trim()
    if (content.length < 10) {
      return wx.showToast({ title: '小结至少写 10 个字', icon: 'none' })
    }
    if (!/^\d{4}-\d{2}$/.test(this.data.period)) {
      return wx.showToast({ title: '月份格式如 2026-11', icon: 'none' })
    }
    if (this.data.saving) return
    this.setData({ saving: true })
    api.saveTrainingNote({
      plan_id: plan.id,
      kind: this.data.kindIndex === 1 ? 'final' : 'monthly',
      period: this.data.period,
      content
    }).then(() => {
      this.setData({ saving: false, content: '' })
      wx.showToast({ title: '小结已提交', icon: 'success' })
      this.fetch()
    }).catch(e => {
      this.setData({ saving: false })
      wx.showToast({ title: e.message || '提交失败', icon: 'none' })
    })
  },

  toChat() {
    wx.navigateTo({ url: '/pages/chat/chat' })
  },

  toQuiz() {
    wx.navigateTo({ url: '/pages/quiz/quiz?count=5' })
  },

  toRisk() {
    wx.navigateTo({ url: '/pages/study/study' })
  },

  retry() {
    this.fetch()
  }
})
