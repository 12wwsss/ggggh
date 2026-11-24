const api = require('../../utils/api.js')

Page({
  data: {
    qGroups: [],
    exams: [],
    learnItems: [],
    risks: [],
    riskOpen: false,
    riskCur: null,
    learnOpen: false,
    learnCur: null,
    learnBusy: false,
    total: 0,
    wrongCount: 0,
    best: 0,
    loading: true,
    err: ''
  },

  onLoad() {
    this.fetch()
  },

  // 考完试回来刷新一下状态
  onShow() {
    if (this.loaded) this.fetch(true)
  },

  fetch(silent) {
    if (!silent) this.setData({ loading: true, err: '' })
    Promise.all([
      api.practiceGroups().catch(() => ({ groups: [] })),
      api.examList().catch(() => ({ items: [] })),
      api.learnItems().catch(() => ({ items: [] })),
      api.risks().catch(() => ({ items: [] }))
    ]).then(([g, e, l, r]) => {
      const qGroups = g.groups || []
      const total = qGroups.reduce((n, x) => n + (x.count || 0), 0)
      this.loaded = true
      this.setData({
        qGroups,
        exams: e.items || [],
        learnItems: l.items || [],
        risks: r.items || [],
        total,
        wrongCount: (wx.getStorageSync('wrongBook') || []).length,
        best: wx.getStorageSync('quizBest') || 0,
        loading: false
      })
    }).catch(err => {
      this.setData({ loading: false, err: err.message })
    })
  },

  /** 点岗位：看这个岗位的全部题目（不抽题、不跳「今日练一练」） */
  practice(e) {
    const job = e.currentTarget.dataset.job
    wx.navigateTo({ url: '/pages/questions/questions?job=' + encodeURIComponent(job) })
  },

  openExam(e) {
    const id = e.currentTarget.dataset.id
    wx.navigateTo({ url: '/pages/exam/exam?id=' + encodeURIComponent(id) })
  },

  goExamList() {
    wx.navigateTo({ url: '/pages/exam/exam' })
  },

  goWrong() {
    wx.navigateTo({ url: '/pages/wrong/wrong' })
  },

  startTest() {
    wx.navigateTo({ url: '/pages/quiz/quiz?count=5' })
  },

  openLearn(e) {
    const id = e.currentTarget.dataset.id
    const it = (this.data.learnItems || []).find(x => String(x.id) === String(id))
    if (!it) return
    this.learnAt = Date.now()
    this.setData({ learnOpen: true, learnCur: it })
  },

  closeLearn() {
    this.setData({ learnOpen: false, learnCur: null })
  },

  noop() {},

  openRisk(e) {
    const id = Number(e.currentTarget.dataset.id)
    const it = (this.data.risks || []).find(x => Number(x.id) === id)
    if (it) this.setData({ riskOpen: true, riskCur: it })
  },

  closeRisk() {
    this.setData({ riskOpen: false, riskCur: null })
  },

  completeLearn() {
    const it = this.data.learnCur
    if (!it || this.data.learnBusy) return
    if (it.status === 'completed') {
      return wx.showToast({ title: '已标记完成', icon: 'none' })
    }
    this.setData({ learnBusy: true })
    const secs = Math.min(3600, Math.max(30, Math.round((Date.now() - (this.learnAt || Date.now())) / 1000)))
    api.learnProgress(it.id, 100, secs).then(() => {
      this.setData({ learnBusy: false, learnOpen: false })
      wx.showToast({ title: '已标记完成', icon: 'success' })
      this.fetch(true)
    }).catch(err => {
      this.setData({ learnBusy: false })
      wx.showToast({ title: err.message, icon: 'none' })
    })
  },

  quizItem() {
    const it = this.data.learnCur
    if (!it) return
    this.setData({ learnOpen: false })
    wx.navigateTo({ url: '/pages/quiz/quiz?c=' + encodeURIComponent(it.category || '') })
  },

  retry() { this.fetch() }
})
