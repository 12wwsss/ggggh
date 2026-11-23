const app = getApp()
const auth = require('../../utils/auth.js')
const api = require('../../utils/api.js')

Page({
  data: {
    statusTop: 44,
    meName: '工友',
    today: '',
    week: '',
    latest: null,
    medias: [],
    cultures: [],
    summary: null,
    loading: true,
    err: '',
    quick: [
      { name: '问一句', icon: 'quick-mic' },
      { name: '答题闯关', icon: 'quick-quiz' },
      { name: '上报问题', icon: 'quick-alert' }
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
      const cultures = (c.items || []).slice(0, 3).map(it => ({
        id: it.id,
        tag: it.tag || '',
        title: it.title || '',
        body: it.body || ''
      }))
      const medias = (m.items || []).map(it => ({
        id: it.id,
        title: it.title,
        cat: it.cat || '资料',
        type: it.type || 'doc',
        url: it.url,
        desc: it.desc || '',
        typeText: it.type === 'video' ? '视频' : (it.type === 'image' ? '图片' : '资料'),
        actionText: it.type === 'video' ? '点击播放' : (it.type === 'image' ? '查看大图' : '打开文档'),
        dateText: String(it.at || '').slice(0, 10),
        sizeText: it.size
          ? (it.size > 1048576 ? (it.size / 1048576).toFixed(1) + ' MB' : Math.round(it.size / 1024) + ' KB')
          : ''
      }))
      this.setData({
        latest: medias[0] || null,
        medias: medias.slice(1, 4),
        cultures,
        summary: s,
        loading: false
      })
      done && done()
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
      done && done()
    })
  },

  onQuick(e) {
    const i = Number(e.currentTarget.dataset.i)
    if (i === 0) wx.navigateTo({ url: '/pages/chat/chat' })
    if (i === 1) wx.navigateTo({ url: '/pages/quiz/quiz' })
    if (i === 2) wx.navigateTo({ url: '/pages/issue/issue' })
  },

  openLatest() {
    const l = this.data.latest
    if (l) wx.navigateTo({ url: '/pages/media/media?id=' + l.id })
    else wx.navigateTo({ url: '/pages/media/media' })
  },

  openMedia(e) {
    const id = e.currentTarget.dataset.id
    wx.navigateTo({ url: '/pages/media/media' + (id ? '?id=' + id : '') })
  },

  toMedia() { wx.navigateTo({ url: '/pages/media/media' }) },

  toStudy() { wx.navigateTo({ url: '/pages/study/study' }) }
})
