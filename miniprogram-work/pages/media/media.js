const api = require('../../utils/api.js')

Page({
  data: {
    items: [],
    cats: ['全部'],
    cat: 0,
    playing: null,     // 当前播放的 url
    playingTitle: '',
    loading: true,
    err: ''
  },

  onLoad(options) {
    // 从首页点进来时带 ?id=，加载完直接打开那条
    this.openId = options && options.id ? String(options.id) : ''
    // 分类来自后台「基础配置」
    api.options().then(o => {
      const cats = ['全部'].concat(o.mediaCats || [])
      this.setData({ cats })
      this.load()
    }).catch(() => this.load())
  },
  onShow() { if (!this.data.items.length) this.load() },
  onPullDownRefresh() { this.load(() => wx.stopPullDownRefresh()) },

  load(done) {
    this.setData({ loading: true, err: '' })
    const c = this.data.cats[this.data.cat]
    const q = c && c !== '全部' ? '?cat=' + encodeURIComponent(c) : ''
    api.media(q).then(res => {
      const items = (res.items || []).map(x => ({
        ...x,
        typeText: x.type === 'video' ? '视频' : x.type === 'image' ? '图片' : '资料',
        sizeText: x.size ? (x.size > 1048576 ? (x.size / 1048576).toFixed(1) + ' MB' : Math.round(x.size / 1024) + ' KB') : '',
        dateText: (x.at || '').slice(0, 10)
      }))
      this.setData({ items, loading: false })
      if (this.openId) {
        const it = items.find(x => String(x.id) === this.openId)
        this.openId = ''
        if (it) this.playItem(it)
      }
      done && done()
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
      done && done()
    })
  },

  pickCat(e) {
    this.setData({ cat: Number(e.currentTarget.dataset.i), playing: null })
    this.load()
  },

  play(e) {
    const it = this.data.items[Number(e.currentTarget.dataset.i)]
    if (it) this.playItem(it)
  },

  playItem(it) {
    if (it.type === 'video') {
      this.setData({ playing: it.url, playingTitle: it.title })
      wx.pageScrollTo({ scrollTop: 0, duration: 200 })
      return
    }

    if (it.type === 'image') {
      wx.previewImage({
        current: it.url,
        urls: [it.url],
        fail: () => wx.showModal({
          title: '图片加载失败',
          content: '请确认手机和服务器在同一网络（当前服务地址：' + it.url + '）',
          showCancel: false
        })
      })
      return
    }

    // 文档：下载后用微信内置阅读器打开（pdf/doc/docx/xls/xlsx/ppt/pptx）
    const ext = (it.url.split('.').pop() || '').toLowerCase()
    const office = ['pdf', 'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx']
    if (office.indexOf(ext) === -1) {
      wx.showModal({
        title: it.title,
        content: '该文件类型（.' + ext + '）微信内无法直接打开，请在电脑上查看。',
        showCancel: false
      })
      return
    }

    wx.showLoading({ title: '下载中', mask: true })
    wx.downloadFile({
      url: it.url,
      success: res => {
        wx.hideLoading()
        if (res.statusCode !== 200) {
          return wx.showToast({ title: '下载失败', icon: 'none' })
        }
        wx.openDocument({
          filePath: res.tempFilePath,
          fileType: ext,
          showMenu: true,
          success: () => {},
          fail: () => wx.showToast({ title: '打开失败', icon: 'none' })
        })
      },
      fail: () => {
        wx.hideLoading()
        wx.showModal({
          title: '下载失败',
          content: '检查手机是否和服务器在同一网络，或稍后重试。',
          showCancel: false
        })
      }
    })
  },

  copyLink() {
    if (!this.data.playing) return
    wx.setClipboardData({
      data: this.data.playing,
      success: () => wx.showToast({ title: '链接已复制', icon: 'none' })
    })
  },

  closePlayer() { this.setData({ playing: null }) }
})
