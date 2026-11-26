const api = require('../../utils/api.js')
const auth = require('../../utils/auth.js')

Page({
  data: {
    p1: '',
    p2: '',
    loading: false,
    err: ''
  },

  onP1(e) { this.setData({ p1: e.detail.value }) },
  onP2(e) { this.setData({ p2: e.detail.value }) },

  submit() {
    const p1 = String(this.data.p1 || '')
    const p2 = String(this.data.p2 || '')
    if (p1.length < 8) return wx.showToast({ title: '新口令至少 8 位', icon: 'none' })
    if (p1 !== p2) return wx.showToast({ title: '两次输入不一致', icon: 'none' })
    this.setData({ loading: true, err: '' })
    api.firstPassword(p1).then(() => {
      const u = auth.user() || {}
      u.must_change_password = false
      auth.setUser(u)
      this.setData({ loading: false })
      wx.showToast({ title: '口令已修改', icon: 'success' })
      setTimeout(() => wx.reLaunch({ url: '/pages/home/home' }), 400)
    }).catch(e => {
      this.setData({ loading: false, err: e.message || '修改失败' })
    })
  }
})
