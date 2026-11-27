const api = require('../../utils/api.js')

function nowPeriod() {
  const d = new Date()
  const m = d.getMonth() + 1
  return d.getFullYear() + '-' + (m < 10 ? '0' + m : m)
}

function planLabel(p) {
  // 师傅身份下要写清带的是谁，否则多个学员分不清
  return p.asMentor ? (p.userName || '学员') + ' · ' + p.title : p.title
}

Page({
  data: {
    plans: [],
    labels: [],
    planIndex: 0,
    plan: null,
    progress: 0,
    notes: [],
    canWrite: false,
    kinds: ['月度小结', '期满总结'],
    kindIndex: 0,
    period: '',
    content: '',
    saving: false,
    stageBusy: false,
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
      const plans = res.plans || []
      const labels = plans.map(planLabel)
      const idx = this.data.planIndex < plans.length ? this.data.planIndex : 0
      this.setData({ plans, labels }, () => this.applyPlan(idx))
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
    })
  },

  applyPlan(idx) {
    const plan = this.data.plans[idx] || null
    if (!plan) {
      this.setData({ plan: null, progress: 0, notes: [], canWrite: false, loading: false })
      return
    }
    const stages = plan.stages || []
    const done = stages.filter(s => s.done).length
    const notes = (plan.notes || []).map(n => ({
      id: n.id,
      kind: n.kind,
      period: n.period,
      content: n.content,
      author: n.author || '—',
      at: n.at || ''
    }))
    this.setData({
      plan,
      planIndex: idx,
      progress: stages.length ? Math.round((done / stages.length) * 100) : 0,
      notes,
      canWrite: !!plan.canWrite,
      loading: false
    })
  },

  onPlanChange(e) {
    this.applyPlan(Number(e.detail.value) || 0)
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

  toggleStage(e) {
    const id = Number(e.currentTarget.dataset.id)
    const done = !e.currentTarget.dataset.done
    if (!id || this.data.stageBusy) return
    this.setData({ stageBusy: true })
    api.toggleTrainingStage({ stage_id: id, done }).then(() => {
      this.setData({ stageBusy: false })
      wx.showToast({ title: done ? '已确认完成' : '已撤销', icon: 'success' })
      this.fetch()
    }).catch(err => {
      this.setData({ stageBusy: false })
      wx.showToast({ title: err.message || '操作失败', icon: 'none' })
    })
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
