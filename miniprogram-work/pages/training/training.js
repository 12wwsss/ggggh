const api = require('../../utils/api.js')

Page({
  data: {
    plan: null,
    progress: 0,
    loading: true,
    err: ''
  },

  onLoad() {
    this.fetch()
  },

  fetch() {
    this.setData({ loading: true, err: '' })
    api.training().then(res => {
      const plan = (res.plans || [])[0] || null
      let progress = 0
      if (plan) {
        const done = plan.stages.filter(s => s.done).length
        progress = Math.round((done / plan.stages.length) * 100)
      }
      this.setData({ plan, progress, loading: false })
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
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
