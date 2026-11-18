const api = require('../../utils/api.js')

Page({
  data: {
    // start | quiz | result
    mode: 'start',
    jobs: [],
    job: '',
    counts: [5, 10],
    count: 5,
    qs: [],
    idx: 0,
    cur: null,
    opts: [],
    picked: -1,
    answered: false,
    score: 0,
    wrong: [],
    spent: 0,
    best: 0
  },

  onLoad(options) {
    // 从「按岗位练题」进来时带 ?job=，等岗位列表回来后再套用
    this.wantJob = options && options.job ? decodeURIComponent(options.job) : ''
    if (options && options.notice) this.noticeId = options.notice
    if (options && options.c) {
      this.cat = decodeURIComponent(options.c)
      this.setData({ catLabel: this.cat })
    }
    const best = wx.getStorageSync('quizBest') || 0
    const patch = { best }
    if (options && options.count) {
      const c = Number(options.count)
      if (c) patch.count = c
    }
    this.setData(patch)
  },

  // 每次回到这一页都重新拉一次岗位：后台改完岗位不用重编译
  onShow() {
    if (this.data.mode === 'start') this.loadJobs()
  },

  pickJob(e) {
    this.setData({ job: e.currentTarget.dataset.j })
  },

  loadJobs() {
    const apply = jobs => {
      let list = ['全部', '通用'].concat((jobs || []).filter(j => j && j !== '通用' && j !== '全部'))
      if (this.wantJob && list.indexOf(this.wantJob) === -1) list = list.concat([this.wantJob])
      this.setData({ jobs: list })

      let job = this.data.job
      if (this.wantJob) {
        job = this.wantJob
        this.wantJob = ''
      } else if (!job || list.indexOf(job) === -1) {
        const last = wx.getStorageSync('quizJob') || ''
        job = (last && list.indexOf(last) > -1) ? last : '全部'
      }
      this.setData({ job })
    }
    // 岗位跟「学习页 · 按岗位练题」同源，保证两边一致
    api.practiceGroups().then(res => {
      const names = (res.groups || []).map(g => g.name).filter(n => n && n !== '通用')
      apply(names)
    }).catch(() => {
      // 老后端没有 by=job，退回基础配置
      api.options().then(o => apply(o.jobs || [])).catch(() => apply([]))
    })
  },

  pickCount(e) {
    this.setData({ count: Number(e.currentTarget.dataset.c) })
  },

  start() {
    const job = this.data.job === '全部' ? '' : this.data.job
    const count = this.data.count
    wx.setStorageSync('quizJob', this.data.job)
    wx.showLoading({ title: '抽题中', mask: true })
    // 优先用后端题库（管理员在后台加的题立即生效）
    api.questions(count, job, this.cat).then(res => {
      wx.hideLoading()
      if (res.items && res.items.length) return this.begin(res.items)
      this.begin(this.localQuestions(job, count))
    }).catch(() => {
      wx.hideLoading()
      this.begin(this.localQuestions(job, count))
    })
  },

  // 断网兜底：用小程序内置的 36 道题
  localQuestions(job, count) {
    const bank = require('../../data/questions.js')
    let pool
    if (job === '通用') {
      pool = bank.filter(q => !q.job || q.job === '通用')
    } else if (job) {
      pool = bank.filter(q => q.job === job || q.job === '通用')
    } else {
      pool = bank.slice()
    }
    for (let i = pool.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [pool[i], pool[j]] = [pool[j], pool[i]]
    }
    return pool.slice(0, Math.min(count, pool.length))
  },

  begin(qs) {
    this.startedAt = Date.now()
    this.setData({ mode: 'quiz', qs, idx: 0, score: 0, wrong: [], spent: 0 })
    this.render()
  },

  render() {
    const { qs, idx } = this.data
    const q = qs[idx]
    if (!q) return
    this.setData({
      cur: q,
      curAns: 'ABCD'[q.a],
      picked: -1,
      answered: false,
      opts: q.o.map((t, i) => ({ t, cls: '', idx: i, key: 'ABCD'[i] }))
    })
  },

  choose(e) {
    if (this.data.answered) return
    const i = Number(e.currentTarget.dataset.i)
    const opts = this.data.opts.map(o => ({ ...o, cls: o.idx === i ? 'opt-on' : '' }))
    this.setData({ picked: i, opts })
  },

  confirm() {
    if (this.data.picked < 0) {
      return wx.showToast({ title: '先选一个答案', icon: 'none' })
    }
    const q = this.data.cur
    const ok = this.data.picked === q.a
    const opts = this.data.opts.map(o => {
      let cls = ''
      if (o.idx === q.a) cls = 'opt-right'
      else if (o.idx === this.data.picked) cls = 'opt-wrong'
      return { ...o, cls }
    })
    const wrong = ok ? this.data.wrong : this.data.wrong.concat([{
      id: q.id, q: q.q, e: q.e, c: q.c,
      your: q.o[this.data.picked], right: q.o[q.a]
    }])
    this.updateWrongBook(q, ok)
    this.setData({
      answered: true,
      opts,
      score: ok ? this.data.score + 1 : this.data.score,
      wrong
    })
  },

  /**
   * 错题本：答错收录（带岗位），答对立刻移出
   * 条目存完整题目，错题集页面可以离线重做
   */
  updateWrongBook(q, ok) {
    if (!q || !q.id) return
    const book = wx.getStorageSync('wrongBook') || []
    const i = book.findIndex(x => x.id === q.id)
    if (ok) {
      if (i > -1) {
        book.splice(i, 1)
        wx.setStorageSync('wrongBook', book)
      }
      return
    }
    const rec = {
      id: q.id, q: q.q, o: q.o, a: q.a, e: q.e, c: q.c,
      job: String(q.job || '').trim(), at: Date.now()
    }
    if (i > -1) {
      rec.times = (book[i].times || 1) + 1
      book[i] = rec
    } else {
      rec.times = 1
      book.push(rec)
    }
    wx.setStorageSync('wrongBook', book.slice(-200))
  },

  next() {
    const { idx, qs } = this.data
    if (idx + 1 >= qs.length) return this.finish()
    this.setData({ idx: idx + 1 })
    this.render()
  },

  // 一个按钮走两种动作：没答先判分，答完进下一题
  onAction() {
    if (this.data.answered) this.next()
    else this.confirm()
  },

  finish() {
    const spent = Math.round((Date.now() - (this.startedAt || Date.now())) / 1000)
    const { score, qs, wrong, job } = this.data
    const best = Math.max(this.data.best, score)
    wx.setStorageSync('quizBest', best)
    wx.setStorageSync('quizCount', (wx.getStorageSync('quizCount') || 0) + 1)

    api.submitQuiz({
      score, total: qs.length, wrong: wrong.map(w => w.id),
      job, spent, notice: this.noticeId || ''
    })
    // 从制度宣贯进来的答题，回写完成率
    if (this.noticeId) {
      api.noticeRead({ id: this.noticeId, team: '综采一队', score, total: qs.length })
    }
    this.setData({ mode: 'result', spent, best })
  },

  again() {
    this.start()
  },

  backHome() {
    wx.reLaunch({ url: '/pages/home/home' })
  }
})
