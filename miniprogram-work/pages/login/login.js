const api = require('../../utils/api.js')
const auth = require('../../utils/auth.js')

Page({
  data: {
    username: '',
    password: '',
    loading: false,
    err: '',
    tip: '',
    server: ''
  },

  onLoad() {
    this.setData({ server: api.serverUrl() })
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
    if (this.data.loading) return
    const username = String(this.data.username || '').trim()
    const password = String(this.data.password || '')
    if (!username || !password) {
      return wx.showToast({ title: '请输入工号和口令', icon: 'none' })
    }
    this.setData({ loading: true, err: '' })
    api.login(username, password).then(res => {
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
      this.setData({ loading: false, err: e.message || '登录失败' })
    })
  },

  /** 真机调试：临时改服务器地址（存本地，不重新编译） */
  editServer() {
    wx.showModal({
      title: '服务器地址',
      editable: true,
      placeholderText: api.serverUrl(),
      content: this.data.server || api.serverUrl(),
      success: r => {
        if (!r.confirm) return
        const v = (r.content || '').trim()
        if (!v) return
        if (!/^https?:\/\//.test(v)) {
          return wx.showToast({ title: '要带 http:// 或 https://', icon: 'none' })
        }
        this.setData({ server: api.setServerUrl(v), err: '' })
        wx.showToast({ title: '已切换服务器', icon: 'success' })
      }
    })
  },

  /** 本机模式：不登录，数据记在手机上 */
  useLocal() {
    wx.reLaunch({ url: '/pages/home/home' })
  }
})
