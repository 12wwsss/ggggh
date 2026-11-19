const api = require('./utils/api.js')
const auth = require('./utils/auth.js')

App({
  globalData: {
    statusBarHeight: 44,
    brand: '煤矿安全智能体',
    loginState: 'device'   // account | device
  },
  onLaunch() {
    const info = wx.getWindowInfo ? wx.getWindowInfo() : wx.getSystemInfoSync()
    this.globalData.statusBarHeight = info.statusBarHeight || 44
    this.globalData.loginState = auth.token() ? 'account' : 'device'
  },
  /** 微信静默登录：拿 openid，换服务端令牌；失败则用本机模式 */
  silentLogin() {
    return new Promise(resolve => {
      const fail = msg => {
        this.globalData.loginState = 'device'
        resolve({ ok: false, error: msg })
      }
      wx.login({
        success: r => {
          if (!r.code) return fail('没拿到 wx.login 的 code')
          api.wxLogin(r.code).then(res => {
            if (res && res.ok && res.token) {
              auth.setToken(res.token)
              auth.setUser(res.user || null)
              this.globalData.loginState = 'wx'
              resolve({ ok: true })
            } else {
              fail((res && (res.error || res.message)) || '服务端没有返回 token')
            }
          }).catch(e => fail(e.message || '请求后端失败'))
        },
        fail: e => fail('wx.login 失败：' + ((e && e.errMsg) || ''))
      })
    })
  }
})
