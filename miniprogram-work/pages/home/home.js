const app = getApp()
const auth = require('../../utils/auth.js')
const api = require('../../utils/api.js')

Page({
  data: {
    statusTop: 44,
    meName: '工友',
    today: '',
    week: '',
    todo: [],
    medias: [],
    cultures: [],
    summary: null,
    loading: true,
    err: '',
    quick: [
      { name: '问一句', icon: 'quick-mic', go: 'chat' },
      { name: '答题', icon: 'quick-quiz', go: 'quiz' },
      { name: '报隐患', icon: 'quick-alert', go: 'issue' },
      { name: '宣贯', icon: 'quick-doc', go: 'policy' }
    ]
  },

  onLoad() {
    const d = new Date()
    const pad = n => (n < 10 ? '0' + n : '' + n)
    const week = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'][d.getDay()]
    this.setData({
      statusTop: app.globalData.statusBarHeight,
      meName: this.displayName(),
      today: pad(d.getMonth() + 1) + '.' + pad(d.getDate()),
      week
    })
    this.load()
  },

  displayName() {
    const u = auth.user() || {}
    return auth.token() && u.name ? u.name : '工友'
  },

  onShow() {
    const u = auth.user() || {}
    if (auth.token() && u.must_change_password) {
      wx.reLaunch({ url: '/pages/password/password' })
      return
    }
    // 同步最新姓名/岗位（管理员改了岗位名称后，手机端不用重新登录）
    if (auth.token()) {
      api.me().then(res => {
        if (res && res.user) {
          auth.setUser(Object.assign({}, res.user, { label: res.user.name }))
          this.setData({ meName: this.displayName() })
        }
      }).catch(() => {})
    }
    this.load()
  },

  onPullDownRefresh() {
    this.load(() => wx.stopPullDownRefresh())
  },

  load(done) {
    this.setData({ loading: true, err: '' })

    Promise.all([
      api.media('').catch(() => ({ items: [] })),
      api.culture().catch(() => ({ items: [] })),
      api.employeeSummary().catch(() => null)
    ]).then(([m, c, s]) => {
      const cultures = (c.items || []).slice(0, 2).map(it => ({
        id: it.id,
        tag: it.tag || it.cat || '',
        title: it.title || '',
        body: Array.isArray(it.body) ? it.body.join(' ') : (it.body || '')
      }))
      const medias = (m.items || []).slice(0, 3).map(it => ({
        id: it.id,
        title: it.title,
        cat: it.cat || '资料',
        type: it.type || 'doc',
        url: it.url,
        desc: it.desc || '',
        icon: it.type === 'video' ? '▶' : (it.type === 'image' ? '图' : '文'),
        dateText: String(it.at || '').slice(0, 10)
      }))
      this.setData({
        medias,
        cultures,
        summary: s,
        todo: this.buildTodo(s),
        loading: false
      })
      done && done()
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
      done && done()
    })
  },

  // 上岗前先看“有没有事要办”：未读消息 / 必修没学完 / 学时没达标。
  // 标题只说干什么，具体数量放右边大号数字，扫一眼就知道差多少。
  buildTodo(s) {
    const todo = []
    if (!s) return todo
    const unread = Number(s.unreadMessages || 0)
    if (unread > 0) {
      todo.push({ key: 'msg', tone: 'warn', num: unread, title: '未读消息', sub: '工单进度、宣贯、带培通知' })
    }
    const reqTotal = Number(s.requiredTotal || 0)
    const reqDone = Number(s.requiredDone || 0)
    if (reqTotal > reqDone) {
      todo.push({
        key: 'required', tone: 'danger', num: reqTotal - reqDone,
        title: '必修没学完', sub: '共 ' + reqTotal + ' 条，已完成 ' + reqDone + ' 条'
      })
    }
    const hours = Number(s.hours || 0)
    const target = Number(s.targetHours || 0)
    if (target > 0 && hours < target) {
      todo.push({
        key: 'hours', tone: 'info', num: target - hours,
        title: '学时没达标', sub: '已学 ' + hours + ' / ' + target + ' 学时'
      })
    }
    return todo.slice(0, 3)
  },

  onQuick(e) {
    const i = Number(e.currentTarget.dataset.i)
    const go = this.data.quick[i] && this.data.quick[i].go
    if (go === 'chat') wx.navigateTo({ url: '/pages/chat/chat' })
    if (go === 'quiz') wx.navigateTo({ url: '/pages/quiz/quiz' })
    if (go === 'issue') wx.navigateTo({ url: '/pages/issue/issue' })
    if (go === 'policy') wx.navigateTo({ url: '/pages/policy/policy' })
  },

  onTodo(e) {
    const i = Number(e.currentTarget.dataset.i)
    const item = this.data.todo[i]
    if (!item) return
    if (item.key === 'msg') return wx.navigateTo({ url: '/pages/messages/messages' })
    wx.navigateTo({ url: '/pages/study/study' })
  },

  openMedia(e) {
    const id = e.currentTarget.dataset.id
    wx.navigateTo({ url: '/pages/media/media' + (id ? '?id=' + id : '') })
  },

  toMedia() { wx.navigateTo({ url: '/pages/media/media' }) },
  toCulture() { wx.navigateTo({ url: '/pages/culture/culture' }) },
  goLearn() { wx.navigateTo({ url: '/pages/study/study' }) },

  openCulture(e) {
    const id = e.currentTarget.dataset.id
    wx.navigateTo({ url: '/pages/article/article?id=' + id })
  }
})
