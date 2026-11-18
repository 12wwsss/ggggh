const api = require('../../utils/api.js')

Page({
  data: {
    id: '',
    item: null,
    loading: true,
    err: ''
  },

  onLoad(options) {
    const id = options.id || ''
    this.setData({ id })
    this.fetch(id)
  },

  fetch(id) {
    this.setData({ loading: true, err: '' })
    api.article(id).then(res => {
      this.setData({ item: res.item, loading: false })
      api.learnRead('art:' + id, res.item.minutes || 2)
      wx.setNavigationBarTitle({ title: res.item.cat || '详情' })
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
    })
  },

  askAI() {
    const q = this.data.item && this.data.item.ask
    wx.navigateTo({ url: '/pages/chat/chat' + (q ? '?q=' + encodeURIComponent(q) : '') })
  },

  toQuiz() {
    wx.navigateTo({ url: '/pages/quiz/quiz' })
  },

  retry() {
    this.fetch(this.data.id)
  }
})
