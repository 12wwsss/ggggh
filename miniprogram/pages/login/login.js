const app = getApp()

Page({
  data: {
    loading: false,
    needBind: false,
    bindTicket: '',
    username: '',
    password: '',
    error: '',
  },

  onLoad() {
    if (app.globalData.token) {
      wx.redirectTo({ url: '/pages/index/index' })
      return
    }
    this.login()
  },

  // 微信登录：已绑定直接进，未绑定转工号绑定
  login() {
    this.setData({ loading: true, error: '' })
    wx.login({
      success: (res) => {
        if (!res.code) {
          this.setData({ loading: false, error: '微信登录失败，请重试' })
          return
        }
        wx.request({
          url: `${app.globalData.base}/api/v1/auth/wx-login`,
          method: 'POST',
          data: { code: res.code },
          success: (r) => {
            const body = r.data || {}
            if (r.statusCode !== 200 || !body.data) {
              this.setData({ loading: false, error: body.message || '登录失败' })
              return
            }
            if (body.data.need_bind) {
              this.setData({ loading: false, needBind: true, bindTicket: body.data.bind_ticket })
              return
            }
            app.setSession(body.data)
            wx.setStorageSync('coalrag_ticket', body.data.ticket || '')
            wx.redirectTo({ url: '/pages/index/index' })
          },
          fail: () => this.setData({ loading: false, error: '网络不可用，请稍后重试' }),
        })
      },
      fail: () => this.setData({ loading: false, error: '微信登录失败，请重试' }),
    })
  },

  // 首次绑定：工号 + 初始口令
  bind() {
    const { bindTicket, username, password } = this.data
    if (!username || !password) {
      this.setData({ error: '请填写工号和口令' })
      return
    }
    this.setData({ loading: true, error: '' })
    wx.request({
      url: `${app.globalData.base}/api/v1/auth/wx-bind`,
      method: 'POST',
      data: { bind_ticket: bindTicket, username, password },
      success: (r) => {
        const body = r.data || {}
        if (r.statusCode !== 200 || !body.data) {
          this.setData({ loading: false, error: body.message || '绑定失败' })
          return
        }
        app.setSession(body.data)
        wx.setStorageSync('coalrag_ticket', body.data.ticket || '')
        wx.redirectTo({ url: '/pages/index/index' })
      },
      fail: () => this.setData({ loading: false, error: '网络不可用，请稍后重试' }),
    })
  },

  onUsername(e) { this.setData({ username: e.detail.value }) },
  onPassword(e) { this.setData({ password: e.detail.value }) },
})
