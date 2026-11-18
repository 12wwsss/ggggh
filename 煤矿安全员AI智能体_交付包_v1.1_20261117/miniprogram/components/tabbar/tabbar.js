Component({
  properties: { active: { type: Number, value: 0 } },
  data: {
    items: [
      { name: '首页', icon: 'tab-home', path: '/pages/home/home' },
      { name: '问答', icon: 'tab-chat', path: '/pages/chat/chat', push: true },
      { name: '学习', icon: 'tab-book', path: '/pages/study/study' },
      { name: '我的', icon: 'tab-user', path: '/pages/me/me' }
    ]
  },
  methods: {
    go(e) {
      const i = Number(e.currentTarget.dataset.i)
      if (i === this.data.active) return
      const item = this.data.items[i]
      if (item.push) {
        wx.navigateTo({ url: item.path, fail: () => wx.reLaunch({ url: item.path }) })
      } else {
        wx.reLaunch({ url: item.path })
      }
    }
  }
})
