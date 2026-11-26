const api = require('../../utils/api.js')
const auth = require('../../utils/auth.js')

Page({
  data: {
    mode: 'list',        // list | run | result
    exams: [],
    myJob: '',
    jobOptions: ['全部'],
    loading: true,
    err: '',
    // 考试中
    qs: [],
    idx: 0,
    cur: null,
    picked: -1,
    answers: [],
    total: 0,
    left: '00:00',
    // 结果
    result: null,
    detail: []
  },

  onLoad(options) {
    this.pendingExamId = options && options.id ? decodeURIComponent(options.id) : ''
    const saved = wx.getStorageSync('workerJob') || ''
    const quizJob = wx.getStorageSync('quizJob') || ''
    const guess = saved || ((quizJob && quizJob !== '全部' && quizJob !== '通用') ? quizJob : '')
    this.setData({ myJob: guess })
    api.options().then(o => {
      this.setData({ jobOptions: (o.jobs || []).slice() })
    }).catch(() => {})
    this.load()
  },

  /** 我的岗位：只用来挡「非本岗位」的考试，选全部＝不限制 */
  pickMyJob(e) {
    const j = e.currentTarget.dataset.j
    const val = j === '全部' ? '' : j
    wx.setStorageSync('workerJob', val)
    this.setData({ myJob: val })
    this.mark()
  },

  mark() {
    const my = this.data.myJob
    const exams = (this.data.exams || []).map(x => Object.assign({}, x, {
      mine: !x.job || !my || x.job === my
    }))
    this.setData({ exams })
  },

  load() {
    this.setData({ loading: true, err: '' })
    api.examList().then(res => {
      this.setData({ exams: res.items || [], loading: false })
      this.mark()
      if (this.pendingExamId) {
        const id = this.pendingExamId
        this.pendingExamId = ''
        this.startById(id)
      }
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
    })
  },

  start(e) {
    this.startById(e.currentTarget.dataset.id)
  },

  startById(id) {    const ex = this.data.exams.find(x => x.id === id)
    if (!ex) return
    if (ex.status === 'done') {
      return wx.showToast({ title: '考试次数已用完', icon: 'none' })
    }
    if (!ex.mine) {
      return wx.showModal({
        title: '不是本岗位的考试',
        content: '这场考试仅限「' + ex.job + '」岗位。你可以在上方把「我的岗位」切到对应岗位再考。',
        showCancel: false
      })
    }
    wx.showModal({
      title: '开始考试',
      content: ex.title + '\n' + ex.questions + ' 题 · ' + ex.minutes + ' 分钟 · ' + ex.pass + ' 分及格\n\n' +
        (ex.passed ? '已通过，本次为重考（剩余 ' + ((ex.max_attempts || 1) - ex.attempts) + ' 次）' : '开始后计时，交卷前可改答案'),
      success: r => { if (r.confirm) this.doStart(id) }
    })
  },

  doStart(id) {
    wx.showLoading({ title: '出卷中', mask: true })
    api.examStart(id, this.data.myJob).then(res => {
      wx.hideLoading()
      this.deadline = res.deadline
      this.sessionId = res.sessionId
      this.setData({
        mode: 'run',
        qs: res.questions,
        total: res.total,
        idx: 0,
        answers: new Array(res.total).fill(-1),
        picked: -1
      })
      this.render()
      this.tick()
    }).catch(e => {
      wx.hideLoading()
      wx.showToast({ title: e.message, icon: 'none' })
    })
  },

  render() {
    const q = this.data.qs[this.data.idx]
    if (!q) return
    this.setData({
      cur: q,
      picked: this.data.answers[this.data.idx],
      opts: q.o.map((t, i) => ({ t, i, key: 'ABCD'[i], cls: this.data.answers[this.data.idx] === i ? 'opt-on' : '' }))
    })
  },

  pick(e) {
    const i = Number(e.currentTarget.dataset.i)
    const answers = this.data.answers.slice()
    answers[this.data.idx] = i
    this.setData({
      answers,
      picked: i,
      opts: this.data.cur.o.map((t, k) => ({ t, i: k, key: 'ABCD'[k], cls: k === i ? 'opt-on' : '' }))
    })
  },

  prev() {
    if (this.data.idx === 0) return
    this.setData({ idx: this.data.idx - 1 })
    this.render()
  },

  next() {
    if (this.data.idx + 1 >= this.data.qs.length) return this.confirmSubmit()
    this.setData({ idx: this.data.idx + 1 })
    this.render()
  },

  // WXML 不支持动态事件名，用这个分发
  onNext() {
    if (this.data.idx + 1 >= this.data.qs.length) this.confirmSubmit()
    else this.next()
  },

  confirmSubmit() {
    const blank = this.data.answers.filter(a => a < 0).length
    wx.showModal({
      title: '交卷',
      content: blank ? '还有 ' + blank + ' 题没答，确定交卷？' : '确定交卷？交卷后不能重考',
      success: r => { if (r.confirm) this.submit() }
    })
  },

  submit() {
    if (this.timer) clearInterval(this.timer)
    wx.showLoading({ title: '判卷中', mask: true })
    api.examSubmit(this.sessionId, this.data.answers).then(res => {
      wx.hideLoading()
      this.syncWrongBook(res.detail || [])
      this.setData({ mode: 'result', result: res.result, detail: res.detail || [] })
    }).catch(e => {
      wx.hideLoading()
      wx.showModal({ title: '交卷失败', content: e.message, showCancel: false })
    })
  },

  /** 考试答错的进错题集；答对的从错题集里移除 */
  syncWrongBook(detail) {
    if (!Array.isArray(detail) || !detail.length) return
    const rightIds = {}
    const wrongs = []
    detail.forEach(d => {
      if (!d.id) return
      api.wrongSet(d.id, !!d.ok)
      if (d.ok) rightIds[d.id] = 1
      else wrongs.push(d)
    })
    let book = (wx.getStorageSync('wrongBook') || []).filter(x => !rightIds[x.id])
    wrongs.forEach(d => {
      const rec = {
        id: d.id, q: d.q, o: d.o, a: d.a, e: d.e, c: d.c,
        job: String(d.job || '').trim(), at: Date.now()
      }
      const i = book.findIndex(x => x.id === d.id)
      if (i > -1) {
        rec.times = (book[i].times || 1) + 1
        book[i] = rec
      } else {
        rec.times = 1
        book.push(rec)
      }
    })
    wx.setStorageSync('wrongBook', book.slice(-200))
  },

  /** 历次成绩：选一次回看答卷 */
  history(e) {
    const paperId = e.currentTarget.dataset.id
    wx.showLoading({ title: '加载记录', mask: true })
    api.examRecords(paperId).then(res => {
      wx.hideLoading()
      const items = res.items || []
      if (!items.length) return wx.showToast({ title: '还没有考试记录', icon: 'none' })
      wx.showActionSheet({
        itemList: items.map(x => '第 ' + x.attempt_no + ' 次 · ' + x.score + ' 分 · ' + (x.passed ? '通过' : '未通过')),
        success: r => {
          const rec = items[r.tapIndex]
          if (rec) this.review(rec.id)
        }
      })
    }).catch(err => {
      wx.hideLoading()
      wx.showToast({ title: err.message, icon: 'none' })
    })
  },

  review(id) {
    wx.showLoading({ title: '加载答卷', mask: true })
    api.examReview(id).then(res => {
      wx.hideLoading()
      const lines = (res.detail || []).map((d, i) =>
        (d.ok ? '✓' : '✗') + ' ' + (i + 1) + '. ' + d.q +
        '\n   你：' + (d.your || '未答') + '　正确：' + ('ABCD'[d.a] || ''))
      wx.showModal({
        title: '得分 ' + (res.result ? res.result.score : '—') + ' 分',
        content: lines.join('\n'),
        showCancel: false
      })
    }).catch(err => {
      wx.hideLoading()
      wx.showToast({ title: err.message, icon: 'none' })
    })
  },

  tick() {
    if (this.timer) clearInterval(this.timer)
    this.timer = setInterval(() => {
      const left = Math.max(0, this.deadline - Date.now())
      const m = String(Math.floor(left / 60000)).padStart(2, '0')
      const s = String(Math.floor((left % 60000) / 1000)).padStart(2, '0')
      this.setData({ left: m + ':' + s })
      if (left <= 0) {
        clearInterval(this.timer)
        wx.showToast({ title: '时间到，自动交卷', icon: 'none' })
        this.submit()
      }
    }, 1000)
  },

  onUnload() {
    if (this.timer) clearInterval(this.timer)
  },

  goQuiz() {
    wx.navigateTo({ url: '/pages/quiz/quiz' })
  },

  back() {
    this.load()
    this.setData({ mode: 'list' })
  }
})
