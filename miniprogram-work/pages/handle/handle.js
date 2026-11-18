const api = require('../../utils/api.js')

Page({
  data: {
    tabs: ['待处置', '已闭环', '全部'],
    tab: 0,
    items: [],
    open: '',
    note: '',
    loading: true,
    err: ''
  },

  onLoad() {
    this.fetch()
  },

  onPullDownRefresh() {
    this.fetch(() => wx.stopPullDownRefresh())
  },

  pickTab(e) {
    this.setData({ tab: Number(e.currentTarget.dataset.i), open: '', note: '' }, () => this.fetch())
  },

  fetch(done) {
    const map = ['待分派', '', '']
    const status = map[this.data.tab] || ''
    this.setData({ loading: true, err: '' })
    api.issues(status).then(res => {
      const cls = { '待分派': 'pending', '处置中': 'doing', '已闭环': 'closed' }
      let items = (res.items || []).map(it => ({ ...it, stCls: cls[it.status] || 'pending' }))
      if (this.data.tab === 1) items = items.filter(i => i.stCls === 'closed')
      this.setData({ items, loading: false })
      done && done()
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
      done && done()
    })
  },

  toggle(e) {
    const id = e.currentTarget.dataset.id
    this.setData({ open: this.data.open === id ? '' : id, note: '' })
  },

  onNote(e) {
    this.setData({ note: e.detail.value })
  },

  submit(e) {
    const id = e.currentTarget.dataset.id
    const note = (this.data.note || '').trim()
    if (note.length < 4) return wx.showToast({ title: '写一句处置说明', icon: 'none' })
    api.handleIssue({ id, note, handler: '安全科' }).then(res => {
      wx.showToast({ title: '状态：' + ((res && res.status) || '已更新'), icon: 'none' })
      this.setData({ open: '', note: '' })
      this.fetch()
    }).catch(err => {
      wx.showToast({ title: err.message, icon: 'none' })
    })
  },

  retry() {
    this.fetch()
  }
})
