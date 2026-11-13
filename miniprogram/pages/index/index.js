const app = getApp()

Page({
  data: {
    src: '',
    failed: false,
  },

  onLoad() {
    const ticket = wx.getStorageSync('coalrag_ticket')
    const base = app.globalData.base
    if (!app.globalData.token) {
      wx.redirectTo({ url: '/pages/login/login' })
      return
    }
    // 一次性票据换会话：H5 打开时自动写入登录态，用户无需再输口令
    const src = ticket ? `${base}/?ticket=${encodeURIComponent(ticket)}` : `${base}/`
    this.setData({ src })
    // 票据只能用一次，用过即清
    wx.removeStorageSync('coalrag_ticket')
  },

  onError() {
    this.setData({ failed: true })
  },

  retry() {
    this.setData({ failed: false })
    this.onLoad()
  },
})
