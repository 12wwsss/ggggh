const api = require('../../utils/api.js')

// 图文与安全文化共用这一页：按分类取顶部渐变色，缺失时退回紫色卡片。
const COVER = { 案例: 'case', 安全月: 'month', 寄语: 'word', 光荣榜: 'star', 知识卡: 'card' }

Page({
  data: {
    id: '',
    item: null,
    loading: true,
    err: ''
  },

  onLoad(options) {
    const id = options.id || ''
    this.setData({ id })
    this.fetch(id)
  },

  fetch(id) {
    this.setData({ loading: true, err: '' })
    api.article(id).then(res => {
      const item = res.item || {}
      item.cover = item.cover || COVER[item.cat] || 'card'
      item.date = String(item.date || '').slice(0, 10)
      this.setData({ item, loading: false })
      api.learnRead('art:' + id, item.minutes || 2)
      wx.setNavigationBarTitle({ title: item.tag || item.cat || '详情' })
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
    })
  },

  askAI() {
    const q = this.data.item && this.data.item.ask
    wx.navigateTo({ url: '/pages/chat/chat' + (q ? '?q=' + encodeURIComponent(q) : '') })
  },

  toQuiz() {
    wx.navigateTo({ url: '/pages/quiz/quiz' })
  },

  retry() {
    this.fetch(this.data.id)
  }
})
