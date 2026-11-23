const api = require('../../utils/api.js')

Page({
  data: {
    mode: 'list',        // list | practice
    book: [],
    jobs: ['全部', '通用'],
    job: '全部',
    list: [],
    total: 0,
    idx: 0,
    cur: null,
    opts: [],
    picked: -1,
    checked: false,
    ok: false,
    btnText: '提交答案',
    curMeta: '',
    rightText: ''
  },

  onLoad() {
    this.jobInited = false
    this.reload()
    this.loadJobs()
  },

  onShow() {
    if (this.data.mode === 'list') {
      this.reload()
      this.loadJobs()
    }
  },

  /** 岗位列表跟后台保持一致，每次进页面都拉一次 */
  loadJobs() {
    api.options().then(o => {
      const jobs = ['全部', '通用'].concat((o.jobs || []).filter(j => j && j !== '通用' && j !== '全部'))
      let job = this.data.job || '全部'
      if (!this.jobInited) {
        const last = wx.getStorageSync('quizJob') || ''
        job = last && jobs.indexOf(last) > -1 ? last : '全部'
        this.jobInited = true
      }
      if (jobs.indexOf(job) === -1) job = '全部'
      this.setData({ jobs, job })
      this.build()
    }).catch(() => {})
  },

  reload() {
    const raw = wx.getStorageSync('wrongBook') || []
    // 老版本只存了题干文字，没法重做；这里顺手清掉，重新答错会再收录
    const book = raw.filter(x => x && Array.isArray(x.o) && x.o.length >= 2 && typeof x.a === 'number')
    if (book.length !== raw.length) wx.setStorageSync('wrongBook', book)
    this.setData({ book })
    this.build()
    // 服务端错题本（换设备不丢）；没登录/离线时退回本地
    api.wrongList().then(res => {
      const remote = (res.items || []).filter(x => x && x.id)
      if (!remote.length) return
      this.setData({ book: remote })
      wx.setStorageSync('wrongBook', remote)
      this.build()
    }).catch(() => {})
  },

  /** 按岗位筛：通用题在所有岗位下都出现，岗位题只在本岗位出现 */
  build() {
    const jobs = this.data.jobs || []
    const job = this.data.job
    const list = (this.data.book || []).filter(it => {
      if (job === '全部') return true
      const j = String(it.job || '').trim()
      const isGeneral = !j || j === '通用' || jobs.indexOf(j) === -1
      if (job === '通用') return isGeneral
      return j === job || isGeneral
    })
    this.setData({ list, total: list.length })
  },

  pickJob(e) {
    this.setData({ job: e.currentTarget.dataset.j })
    this.build()
  },

  startAt(e) {
    const i = Number(e.currentTarget.dataset.i)
    if (!this.data.list[i]) return
    this.setData({ mode: 'practice', idx: i })
    this.render()
  },

  render() {
    const it = this.data.list[this.data.idx]
    if (!it) return this.backToList()
    const opts = (it.o || []).map((t, i) => ({ t, i, k: 'ABCD'[i] || '', cls: '' }))
    this.setData({
      cur: it,
      curMeta: [it.job || '通用', it.c].filter(Boolean).join(' · '),
      rightText: (it.o && typeof it.a === 'number') ? it.o[it.a] : '',
      picked: -1,
      checked: false,
      ok: false,
      btnText: '提交答案',
      opts
    })
  },

  choose(e) {
    if (this.data.checked) return
    const i = Number(e.currentTarget.dataset.i)
    this.setData({
      picked: i,
      opts: this.data.opts.map(o => Object.assign({}, o, { cls: o.i === i ? 'on' : '' }))
    })
  },

  submit() {
    if (this.data.checked) return this.next()
    if (this.data.picked < 0) return wx.showToast({ title: '先选一个答案', icon: 'none' })
    const cur = this.data.cur
    const ok = this.data.picked === cur.a
    const opts = this.data.opts.map(o => Object.assign({}, o, {
      cls: o.i === cur.a ? 'right' : (o.i === this.data.picked ? 'wrong' : '')
    }))
    this.setData({
      checked: true,
      ok,
      opts,
      btnText: (this.data.idx + 1 >= this.data.list.length) ? '完成' : '下一题'
    })
    if (ok) this.removeFromBook(cur.id)
  },

  next() {
    if (this.data.idx + 1 >= this.data.list.length) return this.backToList()
    this.setData({ idx: this.data.idx + 1 })
    this.render()
  },

  backToList() {
    this.setData({ mode: 'list', idx: 0, cur: null })
    this.reload()
  },

  /** 答对即移出错题集 */
  removeFromBook(id) {
    const book = (wx.getStorageSync('wrongBook') || []).filter(x => x.id !== id)
    wx.setStorageSync('wrongBook', book)
    api.wrongSet(id, true)
    this.setData({ book })
    wx.showToast({ title: '答对了，已移出错题集', icon: 'none' })
  }
})
