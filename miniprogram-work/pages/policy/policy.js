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

  /** 宣贯签收 */
  ackNotice(e) {
    const id = e.currentTarget.dataset.id
    const it = (this.data.items || []).find(x => String(x.id) === String(id))
    if (it && it.my_status === 'ack') return
    api.noticeAck(id).then(() => {
      wx.showToast({ title: '已签收', icon: 'success' })
      this.fetch()
    }).catch(err => wx.showToast({ title: err.message, icon: 'none' }))
  },

  retry() {
    this.fetch()
  }
})
