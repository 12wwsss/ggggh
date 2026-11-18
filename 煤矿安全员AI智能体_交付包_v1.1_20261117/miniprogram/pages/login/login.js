const api = require('../../utils/api.js')
const auth = require('../../utils/auth.js')

Page({
  data: {
    loading: false,
    err: '',
    tip: ''
  },

  onLoad() {
    // 已登录直接进首页
    if (auth.token()) {
      wx.reLaunch({ url: '/pages/home/home' })
      return
    }
    // 先静默试一次（服务端配好 appId/appSecret 时无感登录）
    this.trySilent()
  },

  trySilent() {
    const app = getApp()
    app.silentLogin().then(r => {
      if (r && r.ok) wx.reLaunch({ url: '/pages/home/home' })
    }).catch(() => {})
  },

  /** 微信一键登录 */
  login() {
    if (this.data.loading) return
    this.setData({ loading: true, err: '' })
    const app = getApp()
    app.silentLogin().then(r => {
      if (r && r.ok) {
        this.setData({ loading: false })
        wx.showToast({ title: '登录成功', icon: 'success' })
        setTimeout(() => wx.reLaunch({ url: '/pages/home/home' }), 500)
        return
      }
      // 把真实原因显示出来，别再说"缺 appSecret"误导人
      this.setData({
        loading: false,
        err: '微信登录失败：' + ((r && r.error) || '未知原因'),
        tip: '可以先以本机模式使用，记录保存在这台手机上'
      })
    }).catch(e => {
      this.setData({ loading: false, err: e.message || '登录失败' })
    })
  },

  /** 本机模式进入 */
  useLocal() {
    wx.reLaunch({ url: '/pages/home/home' })
  }
})
