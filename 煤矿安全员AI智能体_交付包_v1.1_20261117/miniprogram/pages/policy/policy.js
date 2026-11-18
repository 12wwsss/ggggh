const api = require('../../utils/api.js')

Page({
  data: {
    items: [],
    open: '',
    loading: true,
    err: '',
    stats: null
  },

  onLoad() {
    this.fetch()
  },

  onShow() {
    if (!this.data.loading) this.fetch()
  },

  onPullDownRefresh() {
    this.fetch(() => wx.stopPullDownRefresh())
  },

  fetch(done) {
    this.setData({ loading: true, err: '' })
    api.notices().then(res => {
      this.setData({ items: res.items || [], loading: false })
      done && done()
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
      done && done()
    })
  },

  toggle(e) {
    const id = e.currentTarget.dataset.id
    this.setData({ open: this.data.open === id ? '' : id })
  },

  startQuiz(e) {
    const id = e.currentTarget.dataset.id
    wx.navigateTo({ url: '/pages/quiz/quiz?notice=' + id })
  },

  retry() {
    this.fetch()
  }
})
