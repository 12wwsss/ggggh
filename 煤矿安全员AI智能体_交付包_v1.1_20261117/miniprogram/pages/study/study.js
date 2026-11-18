const api = require('../../utils/api.js')

Page({
  data: {
    qGroups: [],
    exams: [],
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
      api.examList().catch(() => ({ items: [] }))
    ]).then(([g, e]) => {
      const qGroups = g.groups || []
      const total = qGroups.reduce((n, x) => n + (x.count || 0), 0)
      this.loaded = true
      this.setData({
        qGroups,
        exams: e.items || [],
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

  retry() { this.fetch() }
})
