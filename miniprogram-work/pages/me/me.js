const app = getApp()
const api = require('../../utils/api.js')
const auth = require('../../utils/auth.js')

function buildMenus(best, wrongCount, quizCount, s, contact, manager, hasTraining) {
  const sum = s || {}
  const hours = sum.hours || 0
  const pending = (sum.issue && sum.issue.pending) || 0
  const target = sum.targetHours || 72
  const list = [
    { t: '考试与练习', ch: '考', sub: best > 0 ? `最好 ${best} 分 · 已考 ${quizCount} 次` : '在线考试 · 交卷锁定', c: '#E5484D', bg: '#FDECEC', path: '/pages/exam/exam' },
    { t: '错题集', ch: '错', sub: wrongCount ? `${wrongCount} 道待巩固 · 答对自动移出` : '答错自动收录 · 按岗位筛选', c: '#A97900', bg: '#FFF3C4', path: '/pages/wrong/wrong' },
    { t: '学习档案', ch: '学', sub: `学时 ${hours} / ${target}` + (sum.qualified ? ' · 已达标' : ''), c: '#2E6BE6', bg: '#EAF0FD', path: '/pages/study/study' },
    { t: '我的上报', ch: '报', sub: pending ? pending + ' 条待闭环' : '上报隐患 · 查看进度', c: '#F79009', bg: '#FFF6E5', path: '/pages/issue/issue' },
    { t: '消息中心', ch: '信', sub: '待办与通知', c: '#2E6BE6', bg: '#EAF0FD', path: '/pages/messages/messages' },
    { t: '安全科电话', ch: '电', sub: contact || '后台「基础配置」里填写', c: '#A97900', bg: '#FFF3C4', contact: true },
    { t: '帮助与设置', ch: '设', sub: '常见问题 · 清除数据', c: '#7C8B9B', bg: '#EEF1F5', path: '/pages/settings/settings' }
  ]
  if (manager) {
    list.unshift({ t: '问题处理', ch: '处', sub: '受理 · 整改 · 验收 · 闭环', c: '#12B76A', bg: '#E8F7EF', path: '/pages/handle/handle' })
  }
  // 带培入口常驻：没有计划的人也要能看到入口和说明，否则培训员/新员工根本找不到
  list.splice(1, 0, {
    t: '新员工带培',
    ch: '带',
    sub: hasTraining ? '阶段进度 · 师傅小结' : '暂无计划 · 联系师傅或培训员',
    c: '#0E9F6E',
    bg: '#E8F7EF',
    path: '/pages/training/training'
  })
  return list
}

Page({
  data: {
    statusTop: 44,
    stats: [
      { v: '0', lb: '学时' },
      { v: '0', lb: '答题次数' },
      { v: '0', lb: '上报问题' }
    ],
    menus: buildMenus(0, 0, 0, null, '', false, false),
    score: null,
    teamStat: null,
    passRate: 0,
    total: 0,
    contact: '',
    avatar: '',
    todos: [],
    todoTotal: 0
  },

  onLoad() {
    this.setData({
      statusTop: app.globalData.statusBarHeight,
      meName: '本机账号',
      meInitial: '我',
      meInfo: '数据保存在这台手机上'
    })
  },

  /** 身份展示：工号登录优先显示姓名 */
  refreshIdentity() {
    const logged = !!auth.token()
    const u = auth.user()
    const base = logged ? ((u && (u.name || u.label)) || '已登录') : '本机账号'
    const name = this.data.customName || base
    const canHandle = logged && !!u && ['admin', 'safety', 'dept_leader'].indexOf(u.role) >= 0
    this.isManager = canHandle
    this.setData({
      bound: logged,
      canHandle,
      meName: name,
      meInitial: name.slice(0, 1),
      meInfo: logged ? '工号已登录 · 学习记录跟人走' : '未登录 · 记录只在这台手机上'
    })
  },

  /** 点「绑定微信」：走一次静默登录 */
  bindWx() {
    wx.showLoading({ title: '绑定中', mask: true })
    app.silentLogin().then(r => {
      wx.hideLoading()
      if (r && r.ok) {
        this.refreshIdentity()
        wx.showToast({ title: '微信已绑定', icon: 'success' })
      } else {
        wx.showModal({
          title: '绑定失败',
          content: (r && r.error) || '微信登录没有成功，请稍后再试',
          showCancel: false
        })
      }
    }).catch(() => {
      wx.hideLoading()
      wx.showToast({ title: '绑定失败', icon: 'none' })
    })
  },

  /** 点名字：工人自己填姓名/工号，后台就能对上人 */
  editName() {
    const cur = (this.data.customName || '')
    wx.showModal({
      title: '设置姓名',
      editable: true,
      placeholderText: '如：张三 或 采煤-023',
      content: cur,
      success: r => {
        if (!r.confirm) return
        const name = String(r.content || '').trim().slice(0, 20)
        if (name.length < 1) return wx.showToast({ title: '名字不能为空', icon: 'none' })
        api.saveProfile({ name }).then(() => {
          this.setData({ customName: name, meName: name, meInitial: name.slice(0, 1) })
          wx.showToast({ title: '已保存', icon: 'success' })
        }).catch(e => wx.showToast({ title: (e && e.message) || '保存失败', icon: 'none' }))
      }
    })
  },

  onShow() {
    this.refreshIdentity()
    this.loadSummary()
    this.loadScore()
    this.loadContact()
    this.loadProfile()
    this.loadTodos()
    this.loadTraining()
    this.loadMsgBadge()
    const best = wx.getStorageSync('quizBest') || 0
    const wrong = wx.getStorageSync('wrongBook') || []
    const quizCount = wx.getStorageSync('quizCount') || 0
    this._menu = { best, wrong: wrong.length, quiz: quizCount, s: null }
    this.setData({ menus: buildMenus(best, wrong.length, quizCount, null, this.data.contact, this.isManager, this.hasTraining) })
  },

  toHandle() {
    wx.navigateTo({ url: '/pages/handle/handle' })
  },

  goMyIssues() {
    wx.navigateTo({ url: '/pages/issue/issue' })
  },

  goTodos() {
    wx.navigateTo({ url: '/pages/todos/todos' })
  },

  /** 待办提醒：员工看待整改/被驳回，管理层看待受理/待验收/待闭环 */
  loadTodos() {
    if (!auth.token()) return this.setData({ todos: [], todoTotal: 0 })
    api.notifications().then(res => {
      this.setData({ todos: res.items || [], todoTotal: res.total || 0 })
    }).catch(() => {})
  },

  /** 消息中心未读数 */
  loadMsgBadge() {
    if (!auth.token()) return
    api.messages().then(res => {
      const unread = res.unread || 0
      if (!unread) return
      const menus = (this.data.menus || []).map(x =>
        x.path === '/pages/messages/messages' ? { ...x, sub: '未读 ' + unread + ' 条' } : x)
      this.setData({ menus })
    }).catch(() => {})
  },

  /** 有带培计划才显示「新员工带培」入口 */
  loadTraining() {    if (!auth.token()) return
    api.training().then(res => {
      const has = (res.plans || []).length > 0
      if (has !== this.hasTraining) {
        this.hasTraining = has
        this.loadSummary()
      }
    }).catch(() => {})
  },

  /** 头像 / 昵称（跟着微信或设备走） */
  loadProfile() {
    // 手机上优先用本地存的那份（http 图片真机可能加载不出来）
    const local = wx.getStorageSync('avatarLocal') || ''
    if (local) this.setData({ avatar: local })
    api.profile().then(p => {
      const patch = {}
      if (p.avatar && !local) patch.avatar = api.absUrl(p.avatar)
      if (p.name) patch.customName = p.name
      this.setData(patch, () => this.refreshIdentity())
    }).catch(() => {})
  },

  /** 微信头像选择（button open-type="chooseAvatar"） */
  onChooseAvatar(e) {
    const path = e.detail && e.detail.avatarUrl
    if (!path) return
    // 1) 先本地显示，别等网络
    this.setData({ avatar: path })
    // 2) 存一份到手机目录，下次打开还能看到
    try {
      wx.getFileSystemManager().saveFile({
        tempFilePath: path,
        success: r => {
          wx.setStorageSync('avatarLocal', r.savedFilePath)
          this.setData({ avatar: r.savedFilePath })
        },
        fail: () => {}
      })
    } catch (err) {}

    // 3) 再上传给服务器（后台/换设备时用）
    wx.showLoading({ title: '上传中', mask: true })
    wx.getFileSystemManager().readFile({
      filePath: path,
      encoding: 'base64',
      success: r => {
        const ext = (path.split('.').pop() || 'jpg').toLowerCase()
        const mime = ext === 'png' ? 'png' : ext === 'webp' ? 'webp' : 'jpeg'
        api.upload('data:image/' + mime + ';base64,' + r.data).then(up => {
          return api.saveProfile({ avatar: up.url }).then(() => {
            wx.hideLoading()
            wx.showToast({ title: '头像已更新', icon: 'success' })
          })
        }).catch(e2 => {
          wx.hideLoading()
          wx.showToast({ title: '已在本机生效，上传失败：' + ((e2 && e2.message) || ''), icon: 'none' })
        })
      },
      fail: () => { wx.hideLoading(); wx.showToast({ title: '读取头像失败', icon: 'none' }) }
    })
  },

  /** 安全科电话来自后台「基础配置」，改完小程序这边自动变 */
  loadContact() {
    api.options().then(o => {
      const contact = o.contact || ''
      this.setData({ contact })
      const m = this._menu || { best: 0, wrong: 0, quiz: 0, s: null }
      this.setData({ menus: buildMenus(m.best, m.wrong, m.quiz, m.s, contact, this.isManager, this.hasTraining) })
    }).catch(() => {})
  },

  callContact() {
    if (!this.data.contact) {
      return wx.showToast({ title: '后台还没填安全科电话', icon: 'none' })
    }
    wx.makePhoneCall({ phoneNumber: this.data.contact, fail: () => {} })
  },

  loadSummary() {
    api.employeeSummary().then(s => {
      const best = (s.quiz && s.quiz.best) || 0
      const wrong = (wx.getStorageSync('wrongBook') || []).length
      const quiz = (s.quiz && s.quiz.attempts) || 0
      this._menu = { best, wrong, quiz, s }
      this.setData({
        stats: [
          { v: String(s.hours || 0), lb: '学时' },
          { v: String((s.quiz && s.quiz.attempts) || 0), lb: '答题次数' },
          { v: String((s.issue && s.issue.total) || 0), lb: '上报问题' }
        ],
        menus: buildMenus(best, wrong, quiz, s, this.data.contact, this.isManager, this.hasTraining)
      })
    }).catch(() => {})
  },

  loadScore() {
    api.quizStats().then(res => {
      const team = (res.byTeam || [])[0] || null
      this.setData({
        score: res.mine || null,
        teamStat: team,
        passRate: res.passRate,
        total: res.total
      })
    }).catch(() => {})
  },

  open(e) {
    const i = Number(e.currentTarget.dataset.i)
    const m = this.data.menus[i]
    if (!m) return
    if (m.contact) return this.callContact()
    if (m.path) wx.navigateTo({ url: m.path })
  }
})
