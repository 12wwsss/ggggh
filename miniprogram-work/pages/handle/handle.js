const api = require('../../utils/api.js')

/** 后端存的是 UTC 的 ISO 串，显示成本地时间 */
function fmtTime(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return String(iso).slice(0, 16).replace('T', ' ')
  const p = n => (n < 10 ? '0' : '') + n
  return (d.getMonth() + 1) + '-' + p(d.getDate()) + ' ' + p(d.getHours()) + ':' + p(d.getMinutes())
}

Page({
  data: {
    tabs: ['待处置', '已闭环', '全部'],
    tab: 0,
    items: [],
    flows: {},
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
      const act = { reported: '受理', accepted: '提交整改', rectified: '验收通过', verified: '闭环', closed: '已完成' }
      let items = (res.items || []).map(it => ({
        ...it,
        stCls: cls[it.status] || 'pending',
        actText: act[it.raw_status] || '提交',
        canAct: it.raw_status !== 'closed'
      }))
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
    const opening = this.data.open !== id
    this.setData({ open: opening ? id : '', note: '' })
    if (opening && !(this.data.flows || {})[id]) this.loadFlow(id)
  },

  loadFlow(id) {
    api.issueFlow(id).then(res => {
      const flows = Object.assign({}, this.data.flows)
      flows[id] = (res.items || []).map(f => Object.assign({}, f, { atText: fmtTime(f.at) }))
      this.setData({ flows })
    }).catch(() => {})
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
      const flows = Object.assign({}, this.data.flows)
      delete flows[id]
      this.setData({ open: '', note: '', flows })
      this.fetch()
    }).catch(err => {
      wx.showToast({ title: err.message, icon: 'none' })
    })
  },

  retry() {
    this.fetch()
  }
})
