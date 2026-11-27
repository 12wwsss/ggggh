const api = require('../../utils/api.js')

Page({
  data: {
    items: [],
    unread: 0,
    tabs: [
      { label: '全部', kind: '', unread: false },
      { label: '未读', kind: '', unread: true },
      { label: '工单', kind: 'issue' },
      { label: '宣贯', kind: 'broadcast' },
      { label: '带培', kind: 'training' },
      { label: '问答', kind: 'qa' },
      { label: '报表', kind: 'report' }
    ],
    tab: 0,
    loading: true,
    err: ''
  },

  onShow() {
    this.fetch()
  },

  onPullDownRefresh() {
    this.fetch(() => wx.stopPullDownRefresh())
  },

  fetch(done) {
    this.setData({ loading: true, err: '' })
    const t = this.data.tabs[this.data.tab] || {}
    const qs = []
    if (t.kind) qs.push('kind=' + t.kind)
    if (t.unread) qs.push('unread=1')
    const q = qs.length ? '?' + qs.join('&') : ''
    api.messages(q).then(res => {
      this.setData({ items: res.items || [], unread: res.unread || 0, loading: false })
      done && done()
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
      done && done()
    })
  },

  pickTab(e) {
    this.setData({ tab: Number(e.currentTarget.dataset.i) }, () => this.fetch())
  },

  open(e) {
    const it = this.data.items[Number(e.currentTarget.dataset.i)]
    if (!it) return
    if (!it.read) {
      api.messageRead(it.id).then(res => {
        const items = this.data.items.map(x => x.id === it.id ? { ...x, read: true } : x)
        this.setData({ items, unread: res.unread || 0 })
      }).catch(() => {})
    }
    if (it.ref_type === 'issue') wx.navigateTo({ url: '/pages/issue/issue' })
    else if (it.ref_type === 'broadcast') wx.navigateTo({ url: '/pages/policy/policy' })
    else if (it.ref_type === 'training') wx.navigateTo({ url: '/pages/training/training' })
    else if (it.ref_type === 'qa_ticket') {
      wx.showModal({ title: it.title || '安全科回复', content: it.body || '', showCancel: false })
    }
  },

  readAll() {
    if (!this.data.unread) return
    api.messageReadAll().then(() => {
      this.setData({ items: this.data.items.map(x => ({ ...x, read: true })), unread: 0 })
      wx.showToast({ title: '已全部已读', icon: 'none' })
    })
  }
})
