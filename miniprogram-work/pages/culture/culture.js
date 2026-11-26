const api = require('../../utils/api.js')

// 分类与 H5 安全文化页一致；文章类（案例/安全月/知识卡）与文化类（寄语/光荣榜）合并展示。
const CATS = ['案例', '安全月', '寄语', '光荣榜', '知识卡']
const COVER = { 案例: 'case', 安全月: 'month', 寄语: 'word', 光荣榜: 'star', 知识卡: 'card' }

Page({
  data: {
    cats: CATS,
    cat: '',
    items: [],
    skeleton: [1, 2, 3],
    loading: true,
    err: ''
  },

  onLoad() {
    this.all = []
    this.fetch()
  },

  onPullDownRefresh() {
    this.fetch(() => wx.stopPullDownRefresh())
  },

  fetch(done) {
    this.setData({ loading: true, err: '' })
    api.cultureFeed().then(res => {
      this.all = (res.items || []).map(it => ({
        id: it.id,
        cat: it.cat || '知识卡',
        title: it.title || '',
        summary: it.summary || (Array.isArray(it.body) ? it.body[0] : it.body) || '',
        date: String(it.date || '').slice(0, 10),
        minutes: Number(it.minutes || 0),
        cover: COVER[it.cat] || 'card'
      }))
      this.apply(this.data.cat)
      done && done()
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
      done && done()
    })
  },

  // 一次拉全量，切分类只在本地过滤：不闪白、不用等接口
  apply(cat) {
    const items = this.all.filter(it => !cat || it.cat === cat)
    this.setData({ cat, items, loading: false })
  },

  pickCat(e) {
    const c = e.currentTarget.dataset.c || ''
    if (c === this.data.cat) return
    this.apply(c)
  },

  open(e) {
    wx.navigateTo({ url: '/pages/article/article?id=' + e.currentTarget.dataset.id })
  },

  retry() {
    this.fetch()
  }
})
