const api = require('../../utils/api.js')
const auth = require('../../utils/auth.js')

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
    staff: [],
    staffNames: [],
    staffIdx: 0,
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
      const me = auth.user() || {}
      const isMgr = ['admin', 'safety', 'dept_leader'].indexOf(me.role) >= 0
      let items = (res.items || []).map(it => {
        let canAct = false
        if (it.raw_status === 'reported') canAct = isMgr
        else if (it.raw_status === 'accepted') canAct = String(it.assignee_id) === String(me.id) || me.role === 'admin'
        else if (it.raw_status === 'rectified') canAct = isMgr
        else if (it.raw_status === 'verified') canAct = isMgr
        return {
          ...it,
          photos: (it.photos || []).map(p => api.absUrl(p)),
          rectifyPhotos: (it.rectify_photos || []).map(p => api.absUrl(p)),
          stCls: cls[it.status] || 'pending',
          actText: act[it.raw_status] || '提交',
          canAct,
          waitText: it.raw_status === 'accepted' && !canAct ? '等待责任人整改' : ''
        }
      })
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
    if (!opening) return
    const it = (this.data.items || []).find(x => String(x.id) === String(id))
    if (it && it.raw_status === 'reported') this.loadStaff(it)
    if (!(this.data.flows || {})[id]) this.loadFlow(id)
  },

  loadStaff(issue) {
    api.users(issue.dept_id).then(res => {
      const staff = (res.items || []).filter(u => u.username !== 'smokeprobe' && u.role !== 'safety')
      let idx = staff.findIndex(u => String(u.id) === String(issue.reporter_id))
      if (idx < 0) idx = 0
      this.setData({
        staff,
        staffNames: staff.map(u => u.name + (u.post ? '（' + u.post + '）' : '')),
        staffIdx: idx
      })
    }).catch(() => {})
  },

  onAssignee(e) {
    this.setData({ staffIdx: Number(e.detail.value) })
  },

  previewPhoto(e) {
    const src = e.currentTarget.dataset.src
    if (src) wx.previewImage({ urls: [src], current: src })
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

  /** 阻止点击输入区冒泡到卡片，避免一点输入框就收起 */
  noop() {},

  submit(e) {
    const id = e.currentTarget.dataset.id
    const note = (this.data.note || '').trim()
    const it = (this.data.items || []).find(x => String(x.id) === String(id))
    const payload = { id, note, handler: '安全科' }
    if (it && it.raw_status === 'reported') {
      const staff = this.data.staff || []
      const picked = staff[this.data.staffIdx]
      if (!picked) return wx.showToast({ title: '先选择整改人', icon: 'none' })
      payload.assignee_id = picked.id
    }
    if (note.length < 4) return wx.showToast({ title: '写一句处置说明', icon: 'none' })
    api.handleIssue(payload).then(res => {
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
