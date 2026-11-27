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
    wx.hideLoading()
    this._slowTimer = null
    // 已登录直接进首页；首次登录未改密先去设置口令
    if (auth.token()) {
      const u = auth.user() || {}
      wx.reLaunch({ url: u.must_change_password ? '/pages/password/password' : '/pages/home/home' })
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
    // 只有 15 秒内的“真在请求”才拦重复点击；否则自动解锁，避免一次失败后按钮再也点不动
    if (this.data.loading && Date.now() - (this._loginAt || 0) < 15000) return
    if (this.data.loading) this.setData({ loading: false })
    const username = String(this.data.username || '').trim()
    const password = String(this.data.password || '')
    if (!username || !password) {
      return wx.showToast({ title: '请输入工号和口令', icon: 'none' })
    }
    this._loginAt = Date.now()
    this.setData({ loading: true, err: '', tip: '' })
    // 兜底：12 秒还没回来就把按钮解锁，避免真机断点/网络挂起时永远转圈
    if (this._slowTimer) clearTimeout(this._slowTimer)
    this._slowTimer = setTimeout(() => {
      if (!this.data.loading) return
      this.setData({ loading: false, err: '登录超时，检查手机和服务器是否在同一网络后重试' })
    }, 13000)
    api.login(username, password).then(res => {
      if (this._slowTimer) clearTimeout(this._slowTimer)
      auth.setToken(res.token)
      const must = !!(res.must_change_password || (res.user && res.user.must_change_password))
      const user = Object.assign({}, res.user, {
        label: res.user && res.user.name,
        must_change_password: must
      })
      auth.setUser(user)
      const app = getApp()
      if (app && app.globalData) app.globalData.loginState = 'account'
      this.setData({ loading: false })
      wx.showToast({ title: '登录成功', icon: 'success' })
      setTimeout(() => {
        wx.reLaunch({ url: must ? '/pages/password/password' : '/pages/home/home' })
      }, 400)
    }).catch(e => {
      if (this._slowTimer) clearTimeout(this._slowTimer)
      this.setData({ loading: false, err: e.message || '登录失败' })
    })
  },

  onUnload() {
    if (this._slowTimer) clearTimeout(this._slowTimer)
  },

  onShow() {
    // 自愈：上次请求的回调丢了（切后台、热重载）时，别让按钮永远停在 loading
    if (this.data.loading && Date.now() - (this._loginAt || 0) > 15000) {
      this.setData({ loading: false })
    }
  },

  onHide() {
    if (this._slowTimer) clearTimeout(this._slowTimer)
  },

  /** 本机模式：不登录，数据记在手机上 */
  useLocal() {
    wx.reLaunch({ url: '/pages/home/home' })
  }
})
