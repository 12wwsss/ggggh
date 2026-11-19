const api = require('../../utils/api.js')
const auth = require('../../utils/auth.js')

Page({
  data: {
    username: '',
    password: '',
    loading: false,
    err: '',
    tip: ''
  },

  onLoad() {
    // 已登录直接进首页
    if (auth.token()) {
      wx.reLaunch({ url: '/pages/home/home' })
    }
  },

  onUser(e) {
    this.setData({ username: e.detail.value })
  },

  onPwd(e) {
    this.setData({ password: e.detail.value })
  },

  /** 工号 + 口令登录 */
  login() {
    if (this.data.loading) return
    const username = String(this.data.username || '').trim()
    const password = String(this.data.password || '')
    if (!username || !password) {
      return wx.showToast({ title: '请输入工号和口令', icon: 'none' })
    }
    this.setData({ loading: true, err: '' })
    api.login(username, password).then(res => {
      auth.setToken(res.token)
      auth.setUser(Object.assign({}, res.user, { label: res.user && res.user.name }))
      const app = getApp()
      if (app && app.globalData) app.globalData.loginState = 'account'
      this.setData({ loading: false })
      wx.showToast({ title: '登录成功', icon: 'success' })
      setTimeout(() => wx.reLaunch({ url: '/pages/home/home' }), 400)
    }).catch(e => {
      this.setData({ loading: false, err: e.message || '登录失败' })
    })
  },

  /** 本机模式：不登录，数据记在手机上 */
  useLocal() {
    wx.reLaunch({ url: '/pages/home/home' })
  }
})
