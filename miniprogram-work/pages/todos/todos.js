const api = require('../../utils/api.js')

Page({
  data: {
    items: [],
    total: 0,
    loading: true,
    err: ''
  },

  onShow() {
    this.load()
  },

  load() {
    this.setData({ loading: true, err: '' })
    api.notifications().then(res => {
      this.setData({
        items: res.items || [],
        total: res.total || 0,
        loading: false
      })
    }).catch(e => {
      this.setData({ loading: false, err: e.message || '加载失败' })
    })
  },

  open(e) {
    const key = e.currentTarget.dataset.key
    const map = {
      messages: '/pages/messages/messages',
      rectify: '/pages/issue/issue',
      rejected: '/pages/issue/issue',
      accept: '/pages/handle/handle',
      verify: '/pages/handle/handle',
      close: '/pages/handle/handle'
    }
    const url = map[key] || '/pages/messages/messages'
    wx.navigateTo({ url })
  }
})
