// 小程序入口：只需要改 base 和 appid。
App({
  globalData: {
    // 必须是与小程序后台「request 合法域名」一致的 https 域名
    base: 'https://rag.example.com',
    token: '',
    user: null,
  },

  onLaunch() {
    const token = wx.getStorageSync('coalrag_token')
    if (token) this.globalData.token = token
  },

  setSession(data) {
    this.globalData.token = data.token
    this.globalData.user = data.user
    wx.setStorageSync('coalrag_token', data.token)
    wx.setStorageSync('coalrag_user', data.user)
  },
})
