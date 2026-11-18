const api = require('../../utils/api.js')

Page({
  data: {
    cats: ['案例', '安全月', '寄语', '光荣榜', '知识卡'],
    cat: '',
    items: [],
    loading: true,
    err: ''
  },

  onLoad() {
    this.fetch('')
  },

  onPullDownRefresh() {
    this.fetch(this.data.cat, () => wx.stopPullDownRefresh())
  },

  pickCat(e) {
    const c = e.currentTarget.dataset.c
    this.fetch(c)
  },

  fetch(cat, done) {
    this.setData({ loading: true, err: '', cat })
    api.content(cat).then(res => {
      this.setData({ items: res.items || [], loading: false })
      done && done()
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
      done && done()
    })
  },

  open(e) {
    wx.navigateTo({ url: '/pages/article/article?id=' + e.currentTarget.dataset.id })
  },

  retry() {
    this.fetch(this.data.cat)
  }
})
