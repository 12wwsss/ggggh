const auth = require('../../utils/auth.js')
const api = require('../../utils/api.js')

Page({
  data: {
    deviceId: '',
    version: '1.0',
    contact: '',
    faq: [],
    openIdx: -1
  },

  onLoad() {
    const app = getApp()
    this.setData({
      deviceId: auth.deviceId(),
      logged: !!auth.token(),
      loginState: (app && app.globalData.loginState) || (auth.token() ? 'account' : 'device')
    })
    api.faq(8).then(res => {
      this.setData({ faq: res.items || [] })
    }).catch(() => {})

    api.options().then(o => {
      if (o.contact) this.setData({ contact: o.contact })
      if (o.version) this.setData({ version: o.version })
    }).catch(() => {})
  },

  toggle(e) {
    const i = Number(e.currentTarget.dataset.i)
    this.setData({ openIdx: this.data.openIdx === i ? -1 : i })
  },

  clearLocal() {
    wx.showModal({
      title: '清除本机数据',
      content: '会清空本机的学习记录、答题成绩和错题本，且无法恢复。确定继续？',
      confirmText: '确定清除',
      confirmColor: '#E5484D',
      success: r => {
        if (!r.confirm) return
        try {
          wx.clearStorageSync()
        } catch (e) {}
        auth.deviceId()
        wx.showToast({ title: '已清除', icon: 'success' })
        setTimeout(() => wx.reLaunch({ url: '/pages/home/home' }), 800)
      }
    })
  },

  copyDevice() {
    wx.setClipboardData({
      data: this.data.deviceId,
      success: () => wx.showToast({ title: '已复制', icon: 'none' })
    })
  },

  callContact() {
    if (!this.data.contact) {
      return wx.showToast({ title: '安全科电话未配置', icon: 'none' })
    }
    wx.makePhoneCall({ phoneNumber: this.data.contact }).catch(() => {})
  },

  logoutWx() {
    wx.showModal({
      title: '退出登录',
      content: '退出后可以换工号重新登录，学习、考试记录不会丢。',
      success: r => {
        if (!r.confirm) return
        auth.clearLogin()
        const app = getApp()
        if (app && app.globalData) app.globalData.loginState = 'device'
        wx.reLaunch({ url: '/pages/login/login' })
      }
    })
  },

  goAsk() {
    wx.switchTab({ url: '/pages/chat/chat' })
  },

  goIssue() {
    wx.navigateTo({ url: '/pages/issue/issue' })
  }
})
