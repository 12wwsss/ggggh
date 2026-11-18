const api = require('../../utils/api.js')

Page({
  data: {
    job: '',
    items: [],
    open: -1,
    picked: -1,
    checked: false,
    loading: true,
    err: ''
  },

  onLoad(options) {
    const job = options && options.job ? decodeURIComponent(options.job) : ''
    const name = job || '全部'
    this.setData({ job: name })
    wx.setNavigationBarTitle({ title: name + ' · 岗位题库' })
    this.load(job)
  },

  load(job) {
    this.setData({ loading: true, err: '' })
    api.jobQuestions(job).then(res => {
      const items = (res.items || []).map((it, i) => ({
        id: it.id,
        no: i + 1,
        q: it.q,
        e: it.e || '',
        c: it.c || '',
        job: it.job || '通用',
        a: typeof it.a === 'number' ? it.a : -1,
        rightKey: 'ABCD'[it.a] || '',
        opts: (it.o || []).map((t, k) => ({ t, i: k, k: 'ABCD'[k] || '' }))
      }))
      this.setData({ items, open: -1, picked: -1, checked: false, loading: false })
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
    })
  },

  toggle(e) {
    const i = Number(e.currentTarget.dataset.i)
    if (this.data.open === i) {
      return this.setData({ open: -1, picked: -1, checked: false })
    }
    this.setData({ open: i, picked: -1, checked: false })
  },

  choose(e) {
    if (this.data.checked) return
    this.setData({ picked: Number(e.currentTarget.dataset.i) })
  },

  /** 先自己选，再对答案；这里刷题不进错题集 */
  check() {
    if (this.data.picked < 0) {
      return wx.showToast({ title: '先选一个答案', icon: 'none' })
    }
    this.setData({ checked: true })
  },

  askAI(e) {
    const q = e.currentTarget.dataset.q
    wx.navigateTo({ url: '/pages/chat/chat?ask=' + encodeURIComponent(q) })
  }
})
