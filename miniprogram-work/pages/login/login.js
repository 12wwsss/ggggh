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
    wx.hideLoading()
    this.setData({ server: api.serverUrl() })
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
      this.setData({ loading: false, err: '请求 13 秒没响应（' + api.serverUrl() + '），可点下方「网络诊断」看服务器是否可达' })
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

  /** 网络诊断：直接打 /api/health，区分「服务器不可达」和「账号/口令问题」 */
  diagnose() {
    if (this._diagBusy) return
    this._diagBusy = true
    wx.showLoading({ title: '检测中', mask: true })
    const started = Date.now()
    const finish = () => {
      this._diagBusy = false
      wx.hideLoading()
    }
    const timeout = new Promise((_, reject) => setTimeout(() => reject(new Error('6 秒无响应')), 6000))
    Promise.race([api.health(), timeout]).then(res => {
      finish()
      wx.showModal({
        title: '服务器可达',
        content: api.serverUrl() + '\n耗时 ' + (Date.now() - started) + 'ms · 状态 ' + ((res && res.status) || 'ok') +
          '\n\n能通但登录超时，多半是账号接口问题，把时间点告诉管理员查日志。',
        showCancel: false
      })
    }).catch(e => {
      finish()
      wx.showModal({
        title: '连不上服务器',
        content: api.serverUrl() + '\n' + (e.message || '') +
          '\n\n如果手机浏览器能打开这个地址，但这里失败，是微信的域名校验没过：' +
          '开发者工具「详情 → 本地设置」勾选「不校验合法域名」，再结束调试重新进。',
        showCancel: false
      })
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
