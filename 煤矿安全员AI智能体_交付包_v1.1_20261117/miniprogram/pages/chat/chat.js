const api = require('../../utils/api.js')

Page({
  data: {
    msgs: [],
    input: '',
    recording: false,
    voiceText: '',
    voiceHint: '',
    loading: false,
    err: '',
    scrollTop: 0,
    greet: '我是安全智能体小安。规程怎么规定、这活儿怎么干、隐患怎么判，直接问我，答案都带规程出处。',
    suggests: ['瓦斯超限怎么办？', '支架检修要泄压吗？', '斜巷行车能走人吗？'],
    lastQ: '',
    pending: null
  },

  onLoad(options) {
    this.seq = 1
    this.setData({
      msgs: [{ id: 0, role: 'ai', text: this.data.greet, cites: [], typing: false, greet: true }]
    }, () => {
      // 从文章/风险点带问题进来，直接提问
      if (options && options.q) this.send(options.q)
    })
  },

  onUnload() {
    if (this.timer) clearInterval(this.timer)
  },

  onInput(e) {
    this.setData({ input: e.detail.value })
  },

  tapSuggest(e) {
    this.send(e.currentTarget.dataset.q)
  },

  send(e) {
    const q = typeof e === 'string' ? e : (this.data.input || '').trim()
    if (this.data.loading) return
    // 有图片：必须先写一句问题再发，避免只丢张图过来
    if (this.data.pending) {
      if (!q) return wx.showToast({ title: '先写下要问的问题', icon: 'none' })
      const p = this.data.pending
      this.setData({ pending: null })
      return this.askWithImage(p.url, p.local, q)
    }
    if (!q) return wx.showToast({ title: '先输入问题', icon: 'none' })
    const msgs = this.data.msgs.concat([{ id: this.seq++, role: 'user', text: q, cites: [] }])
    this.setData({ msgs, input: '', loading: true, err: '', lastQ: q }, () => {
      this.scrollToBottom()
      this.ask(q)
    })
  },

  /** 初始化语音识别（微信同声传译插件），不可用时退回录音模式 */
  initVoice() {
    // 插件需在小程序后台添加后才可用；未添加时直接走录音模式
    let canUsePlugin = false
    try { canUsePlugin = typeof requirePlugin === 'function' && !!wx.getSystemInfoSync().SDKVersion } catch (e) { canUsePlugin = false }
    if (!canUsePlugin) {
      this.voiceReady = false
      this.setData({ voiceHint: '长按录音并发送（添加同声传译插件后可转文字）' })
      return
    }
    try {
      const plugin = requirePlugin('WechatSI')
      this.siPlugin = plugin
      const mgr = plugin.getRecordRecognitionManager()
      this.siMgr = mgr
      mgr.onRecognize = res => {
        if (res.result) this.setData({ input: res.result })
      }
      mgr.onStop = res => {
        this.setData({ recording: false })
        const text = (res.result || '').trim()
        if (text) {
          this.setData({ input: text })
          wx.showToast({ title: '识别完成，可修改后发送', icon: 'none' })
        } else {
          wx.showToast({ title: '没听清，再说一次', icon: 'none' })
        }
      }
      mgr.onError = () => {
        this.setData({ recording: false })
        wx.showToast({ title: '语音识别失败', icon: 'none' })
      }
      this.voiceReady = true
      this.setData({ voiceHint: '长按说话，松开变文字' })
    } catch (e) {
      this.voiceReady = false
      this.setData({ voiceHint: '长按录音（需在后台添加同声传译插件可转文字）' })
    }
  },

  /** 选图并上传，附在问题里问 AI */
  pickImage() {
    wx.chooseMedia({
      count: 1,
      mediaType: ['image'],
      sizeType: ['compressed'],
      success: res => {
        const path = res.tempFiles[0].tempFilePath
        wx.showLoading({ title: '上传中', mask: true })
        wx.compressImage({
          src: path, quality: 60,
          success: r => this.sendImg(r.tempFilePath || path, path),
          fail: () => this.sendImg(path, path)
        })
      }
    })
  },

  sendImg(path, localPath) {
    wx.getFileSystemManager().readFile({
      filePath: path,
      encoding: 'base64',
      success: r => {
        const ext = (path.split('.').pop() || 'jpg').toLowerCase()
        const mime = ext === 'png' ? 'png' : ext === 'webp' ? 'webp' : 'jpeg'
        api.upload('data:image/' + mime + ';base64,' + r.data).then(up => {
          wx.hideLoading()
          // 只挂到输入框上方，等员工写问题再发
          this.setData({ pending: { url: up.url, local: localPath || path } })
          wx.showToast({ title: '图片已添加，写下要问的问题', icon: 'none', duration: 2200 })
        }).catch(e => {
          wx.hideLoading()
          wx.showToast({ title: e.message || '上传失败', icon: 'none' })
        })
      },
      fail: () => { wx.hideLoading(); wx.showToast({ title: '读图失败', icon: 'none' }) }
    })
  },

  askWithImage(url, localPath, text) {
    const q = String(text || '').trim()
    if (!q) return wx.showToast({ title: '先写下要问的问题', icon: 'none' })
    const msgs = this.data.msgs.concat([{
      id: this.seq++, role: 'user', text: q, cites: [], image: localPath
    }])
    this.setData({ msgs, input: '', loading: true, err: '' }, () => {
      this.scrollToBottom()
      // 图片由后端转成 base64 交给模型，模型能看图
      this.ask(q, url)
    })
  },

  /** 麦克风权限：没授权先申请，被拒过就引导去设置里开 */
  ensureRecordAuth() {
    return new Promise(resolve => {
      wx.getSetting({
        success: r => {
          const st = (r.authSetting || {})['scope.record']
          if (st === true) return resolve(true)
          if (st === false) return this.askOpenSetting(resolve)
          wx.authorize({
            scope: 'scope.record',
            success: () => resolve(true),
            fail: () => this.askOpenSetting(resolve)
          })
        },
        fail: () => resolve(true)
      })
    })
  },

  askOpenSetting(resolve) {
    wx.showModal({
      title: '需要麦克风权限',
      content: '语音提问要用麦克风。点「去设置」，把「录音」打开，回来再长按一次「语音」。',
      confirmText: '去设置',
      success: r => {
        if (!r.confirm) return resolve(false)
        wx.openSetting({
          success: s => resolve(!!((s.authSetting || {})['scope.record'])),
          fail: () => resolve(false)
        })
      },
      fail: () => resolve(false)
    })
  },

  /** 长按录音，松开发送 */
  startVoice() {
    if (this.data.recording) return
    // 已授权：直接开始录；没授权：弹窗申请/引导去设置，授权完让用户再长按一次
    wx.getSetting({
      success: r => {
        const st = (r.authSetting || {})['scope.record']
        if (st === true) return this.beginRecord()
        this.ensureRecordAuth().then(ok => {
          if (ok) {
            wx.showToast({ title: '麦克风已开启，再长按「语音」说一次', icon: 'none', duration: 2200 })
          } else {
            wx.showToast({ title: '没有麦克风权限，语音用不了', icon: 'none' })
          }
        })
      },
      fail: () => this.beginRecord()
    })
  },

  beginRecord() {
    // 有识别插件：边说边转文字
    if (this.voiceReady && this.siMgr) {
      this.setData({ recording: true, input: '' })
      this.siMgr.start({ lang: 'zh_CN', duration: 60000 })
      wx.showToast({ title: '正在听…松开结束', icon: 'none', duration: 1500 })
      return
    }
    const rec = wx.getRecorderManager()
    this.recorder = rec
    rec.start({ duration: 60000, format: 'mp3', sampleRate: 16000, numberOfChannels: 1 })
    this.setData({ recording: true })
    wx.showToast({ title: '松开发送', icon: 'none', duration: 1500 })
    rec.onStop(res => {
      if (res.tempFilePath) this.sendVoice(res.tempFilePath, Math.round((res.duration || 0) / 1000))
    })
  },

  stopVoice() {
    if (!this.data.recording) return
    if (this.voiceReady && this.siMgr) {
      this.siMgr.stop()
      return
    }
    this.setData({ recording: false })
    if (this.recorder) this.recorder.stop()
  },

  sendVoice(path, sec) {
    if (!sec) return
    wx.showLoading({ title: '上传中', mask: true })
    wx.getFileSystemManager().readFile({
      filePath: path,
      encoding: 'base64',
      success: r => {
        api.upload('data:audio/mp3;base64,' + r.data).then(up => {
          wx.hideLoading()
          const msgs = this.data.msgs.concat([{
            id: this.seq++, role: 'user', text: '（语音消息 ' + sec + ' 秒）', cites: [],
            audio: up.url, audioSec: sec
          }])
          this.setData({ msgs }, () => {
            this.scrollToBottom()
            wx.showToast({ title: '语音已发送，可补充文字让 AI 回答', icon: 'none', duration: 2500 })
          })
        }).catch(e => {
          wx.hideLoading()
          wx.showToast({ title: e.message || '上传失败', icon: 'none' })
        })
      },
      fail: () => { wx.hideLoading(); wx.showToast({ title: '读音频失败', icon: 'none' }) }
    })
  },

  playAudio(e) {
    const raw = e.currentTarget.dataset.src
    if (!raw) return
    // /uploads/... 是相对路径，要拼完整地址；插件返回的临时路径原样用
    const src = raw.charAt(0) === '/' ? api.absUrl(raw) : raw
    if (!this.audioCtx) this.audioCtx = wx.createInnerAudioContext()
    this.audioCtx.src = src
    this.audioCtx.play()
  },

  previewImg(e) {
    const src = e.currentTarget.dataset.src
    if (src) wx.previewImage({ urls: [src], current: src })
  },

  previewPending() {
    const p = this.data.pending
    if (p && p.local) wx.previewImage({ urls: [p.local], current: p.local })
  },

  removePending() {
    this.setData({ pending: null })
  },

  scrollToBottom() {
    this.setData({ scrollTop: (this.data.scrollTop || 0) + 4000 })
  },

  ask(q, image) {
    // 只带最近 3 轮，控制 token
    const history = this.data.msgs
      .filter(m => !m.greet)
      .slice(-6)
      .map(m => ({ role: m.role, content: m.text }))

    api.chat(q, history, image).then(res => {
      const idx = this.data.msgs.length
      const msgs = this.data.msgs.concat([{
        id: this.seq++, role: 'ai', text: '', full: res.answer,
      cites: res.cites || [], grounded: res.grounded !== false, typing: true
      }])
      this.setData({ msgs, loading: false }, () => {
        this.scrollToBottom()
        this.typewriter(idx)
      })
    }).catch(e => {
      this.setData({ loading: false, err: e.message })
    })
  },

  typewriter(idx) {
    const full = this.data.msgs[idx].full || ''
    let n = 0
    const step = Math.max(2, Math.ceil(full.length / 90))
    if (this.timer) clearInterval(this.timer)
    this.timer = setInterval(() => {
      n += step
      if (n >= full.length) {
        clearInterval(this.timer)
        this.timer = null
        this.setData({
          ['msgs[' + idx + '].text']: full,
          ['msgs[' + idx + '].typing']: false
        }, () => this.scrollToBottom())
        return
      }
      this.setData({ ['msgs[' + idx + '].text']: full.slice(0, n) })
    }, 26)
  },

  retry() {
    if (this.data.lastQ) this.ask(this.data.lastQ)
  },

  toHuman() {
    wx.showToast({ title: '已转当班安全员', icon: 'none' })
  },

  /** 朗读最后一条 AI 回答（同声传译插件 TTS） */
  voice() {
    const ai = (this.data.msgs || []).filter(m => m.role === 'ai' && m.full).pop()
    if (!ai) return wx.showToast({ title: '还没有可朗读的回答', icon: 'none' })
    if (!this.siPlugin || !this.siPlugin.textToSpeech) {
      return wx.showModal({
        title: '朗读不可用',
        content: '需要在微信公众平台「设置 → 第三方设置 → 插件管理」添加「微信同声传译」插件，然后再编译一次。',
        showCancel: false
      })
    }
    const text = String(ai.full).slice(0, 300)
    wx.showLoading({ title: '合成中', mask: true })
    this.siPlugin.textToSpeech({
      lang: 'zh_CN',
      tts: true,
      content: text,
      success: res => {
        wx.hideLoading()
        if (!this.audioCtx) this.audioCtx = wx.createInnerAudioContext()
        this.audioCtx.src = res.filename
        this.audioCtx.play()
      },
      fail: () => {
        wx.hideLoading()
        wx.showToast({ title: '语音合成失败', icon: 'none' })
      }
    })
  }
})
