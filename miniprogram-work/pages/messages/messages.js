const api = require('../../utils/api.js')

Page({
  data: {
    items: [],
    unread: 0,
    loading: true,
    err: ''
  },

  onShow() {
    this.fetch()
  },

  onPullDownRefresh() {
    this.fetch(() => wx.stopPullDownRefresh())
  },

  fetch(done) {
    this.setData({ loading: true, err: '' })
    api.messages().then(res => {
      this.setData({ items: res.items || [], unread: res.unread || 0, loading: false })
      done && done()
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
      done && done()
    })
  },

  open(e) {
    const it = this.data.items[Number(e.currentTarget.dataset.i)]
    if (!it) return
    if (!it.read) {
      api.messageRead(it.id).then(res => {
        const items = this.data.items.map(x => x.id === it.id ? { ...x, read: true } : x)
        this.setData({ items, unread: res.unread || 0 })
      }).catch(() => {})
    }
    if (it.ref_type === 'issue') wx.navigateTo({ url: '/pages/issue/issue' })
    else if (it.ref_type === 'broadcast') wx.navigateTo({ url: '/pages/policy/policy' })
    else if (it.ref_type === 'training') wx.navigateTo({ url: '/pages/training/training' })
  },

  readAll() {
    if (!this.data.unread) return
    api.messageReadAll().then(() => {
      this.setData({ items: this.data.items.map(x => ({ ...x, read: true })), unread: 0 })
      wx.showToast({ title: '已全部已读', icon: 'none' })
    })
  }
})
