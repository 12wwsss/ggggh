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
    types: ['安全隐患', '设备故障', '制度疑问', '其他'],
    type: 0,
    urgent: 0,
    desc: '',
    loc: '',
    photos: [],
    allItems: [],
    items: [],
    tabs: ['全部', '我上报的', '指派给我'],
    tab: 0,
    submitting: false,
    recording: false,
    audio: '',
    audioSec: 0,
    voiceTip: '',
    voiceMode: 'audio',   // audio=录音发送  text=边说边转文字
    audioPlaying: false
  },

  onLoad() {
    this.initVoice()
    this.fetchMine()
  },

  /** 语音：插件可用就转文字，不可用就录音随上报一起提交 */
  initVoice() {
    this.siReady = false
    try {
      const plugin = requirePlugin('WechatSI')
      const mgr = plugin.getRecordRecognitionManager()
      this.siMgr = mgr
      mgr.onStop = res => {
        this.setData({ recording: false })
        const text = (res.result || '').trim()
        if (!text) return wx.showToast({ title: '没听清，再说一次', icon: 'none' })
        const desc = (this.data.desc || '').trim()
        this.setData({ desc: (desc ? desc + ' ' : '') + text })
        wx.showToast({ title: '已转成文字', icon: 'none' })
      }
      mgr.onError = () => {
        this.setData({ recording: false })
        wx.showToast({ title: '识别失败，可改用录音', icon: 'none' })
      }
      this.siReady = true
      this.setData({ voiceTip: '长按说话，边说边出字' })
    } catch (e) {
      this.siReady = false
      this.setData({ voiceTip: '长按录音，随上报一起提交给安全科' })
    }
  },

  /** 切换「录音发送 / 转文字」 */
  pickVoiceMode(e) {
    const mode = e.currentTarget.dataset.mode
    if (mode === 'text' && !this.siReady) {
      wx.showModal({
        title: '转文字还没开通',
        content: '需要甲方在小程序后台添加「微信同声传译」插件。现在可以先用「语音」模式，录好随上报一起提交。',
        showCancel: false
      })
      return
    }
    this.setData({
      voiceMode: mode,
      voiceTip: mode === 'text' ? '长按说话，边说边出字' : '长按录音，随上报一起提交给安全科'
    })
  },

  startVoice() {
    if (this.data.recording) return
    wx.getSetting({
      success: r => {
        const st = (r.authSetting || {})['scope.record']
        if (st === true) return this.beginRecord()
        this.ensureRecordAuth().then(ok => {
          if (ok) wx.showToast({ title: '麦克风已开，再长按一次', icon: 'none' })
          else wx.showToast({ title: '没有麦克风权限', icon: 'none' })
        })
      },
      fail: () => this.beginRecord()
    })
  },

  ensureRecordAuth() {
    return new Promise(resolve => {
      wx.getSetting({
        success: r => {
          const st = (r.authSetting || {})['scope.record']
          if (st === true) return resolve(true)
          const ask = () => wx.openSetting({
            success: s => resolve(!!((s.authSetting || {})['scope.record'])),
            fail: () => resolve(false)
          })
          if (st === false) {
            return wx.showModal({
              title: '需要麦克风权限',
              content: '点「去设置」打开录音权限，回来再长按一次。',
              confirmText: '去设置',
              success: m => m.confirm ? ask() : resolve(false)
            })
          }
          wx.authorize({ scope: 'scope.record', success: () => resolve(true), fail: ask })
        },
        fail: () => resolve(true)
      })
    })
  },

  beginRecord() {
    // 文字模式：插件边说边识别，结果写进描述框
    if (this.data.voiceMode === 'text' && this.siReady && this.siMgr) {
      this.setData({ recording: true })
      this.siMgr.start({ lang: 'zh_CN', duration: 60000 })
      wx.showToast({ title: '正在听…松开结束', icon: 'none', duration: 1500 })
      return
    }
    // 语音模式：录 mp3，挂到上报单上
    const rec = wx.getRecorderManager()
    this.recorder = rec
    rec.onStop(res => {
      this.setData({ recording: false })
      if (res.tempFilePath) this.uploadAudio(res.tempFilePath, Math.round((res.duration || 0) / 1000))
    })
    rec.start({ duration: 60000, format: 'mp3', sampleRate: 16000, numberOfChannels: 1 })
    this.setData({ recording: true })
    wx.showToast({ title: '松开发送', icon: 'none', duration: 1500 })
  },

  stopVoice() {
    if (!this.data.recording) return
    if (this.data.voiceMode === 'text' && this.siReady && this.siMgr) return this.siMgr.stop()
    this.setData({ recording: false })
    if (this.recorder) this.recorder.stop()
  },

  uploadAudio(path, sec) {
    wx.showLoading({ title: '上传语音', mask: true })
    wx.getFileSystemManager().readFile({
      filePath: path,
      encoding: 'base64',
      success: r => {
        api.upload('data:audio/mp3;base64,' + r.data).then(up => {
          wx.hideLoading()
          this.setData({ audio: up.url, audioSec: sec })
          wx.showToast({ title: '语音已附上', icon: 'none' })
        }).catch(e => {
          wx.hideLoading()
          wx.showToast({ title: e.message || '上传失败', icon: 'none' })
        })
      },
      fail: () => { wx.hideLoading(); wx.showToast({ title: '读音频失败', icon: 'none' }) }
    })
  },

  /** 点击播放/暂停，播放中图标变成暂停 */
  playAudio() {
    if (!this.data.audio) return
    if (!this.audioCtx) {
      this.audioCtx = wx.createInnerAudioContext()
      const stop = () => this.setData({ audioPlaying: false })
      this.audioCtx.onEnded(stop)
      this.audioCtx.onStop(stop)
      this.audioCtx.onError(stop)
      this.audioCtx.onPause(stop)
    }
    if (this.data.audioPlaying) {
      this.audioCtx.pause()
      this.setData({ audioPlaying: false })
      return
    }
    const src = api.absUrl(this.data.audio)
    if (this.audioSrc !== src) {
      this.audioSrc = src
      this.audioCtx.src = src
    }
    this.audioCtx.play()
    this.setData({ audioPlaying: true })
  },

  /** 已提交工单里回放自己的语音 */
  playItemAudio(e) {
    const src = e.currentTarget.dataset.src
    if (!src) return
    if (!this.itemAudioCtx) this.itemAudioCtx = wx.createInnerAudioContext()
    this.itemAudioCtx.src = api.absUrl(src)
    this.itemAudioCtx.play()
  },

  removeAudio() {
    if (this.audioCtx) { try { this.audioCtx.stop() } catch (e) {} }
    this.setData({ audio: '', audioSec: 0, audioPlaying: false })
  },

  onShow() {
    this.fetchMine()
  },

  fetchMine() {
    api.issues().then(res => {
      const me = auth.user() || {}
      const cls = { '待分派': 'pending', '处置中': 'doing', '已闭环': 'closed' }
      const all = (res.items || []).map(it => ({
        ...it,
        stCls: cls[it.status] || 'pending',
        isMine: String(it.reporter_id) === String(me.id),
        isAssigned: String(it.assignee_id) === String(me.id),
        canRectify: String(it.assignee_id) === String(me.id) && it.raw_status === 'accepted',
        createdAt: fmtTime(it.createdAt)
      }))
      this.setData({ allItems: all }, () => this.applyTab())
    }).catch(() => {})
  },

  applyTab() {
    const tab = Number(this.data.tab) || 0
    const all = this.data.allItems || []
    let items = all
    if (tab === 1) items = all.filter(x => x.isMine)
    if (tab === 2) items = all.filter(x => x.isAssigned)
    this.setData({ items: items.slice(0, 20) })
  },

  pickTab(e) {
    this.setData({ tab: Number(e.currentTarget.dataset.i) }, () => this.applyTab())
  },

  /** 被指派人提交整改 */
  rectify(e) {
    const id = e.currentTarget.dataset.id
    wx.showModal({
      title: '提交整改',
      editable: true,
      placeholderText: '写清整改结果，例如：已补液加固，现场复测合格',
      success: r => {
        if (!r.confirm) return
        const result = String(r.content || '').trim()
        if (result.length < 2) return wx.showToast({ title: '写一句整改结果', icon: 'none' })
        api.issueRectify({ id, result }).then(res => {
          wx.showToast({ title: '状态：' + ((res && res.status) || '已更新'), icon: 'none' })
          this.fetchMine()
        }).catch(err => wx.showToast({ title: err.message, icon: 'none' }))
      }
    })
  },

  pickType(e) {
    this.setData({ type: Number(e.currentTarget.dataset.i) })
  },

  pickUrgent(e) {
    this.setData({ urgent: Number(e.currentTarget.dataset.i) })
  },

  onDesc(e) {
    this.setData({ desc: e.detail.value })
  },

  /** 选图并真实上传 */
  choosePhoto() {
    if (this.data.photos.length >= 3) {
      return wx.showToast({ title: '最多 3 张', icon: 'none' })
    }
    wx.chooseMedia({
      count: 3 - this.data.photos.length,
      mediaType: ['image'],
      sizeType: ['compressed'],
      success: res => {
        res.tempFiles.forEach(f => this.upload(f.tempFilePath))
      }
    })
  },

  upload(path) {
    // 手机拍的图动辄 2–5MB，先压到 60% 质量再传，避免请求体过大
    wx.showLoading({ title: '上传中', mask: true })
    wx.compressImage({
      src: path,
      quality: 60,
      success: r => this.sendImg(r.tempFilePath || path),
      fail: () => this.sendImg(path)
    })
  },

  sendImg(path) {
    wx.getFileSystemManager().readFile({
      filePath: path,
      encoding: 'base64',
      success: r => {
        const ext = (path.split('.').pop() || 'jpg').toLowerCase()
        const mime = ext === 'png' ? 'png' : ext === 'webp' ? 'webp' : 'jpeg'
        api.upload('data:image/' + mime + ';base64,' + r.data).then(res => {
          wx.hideLoading()
          this.setData({ photos: this.data.photos.concat([api.absUrl(res.url)]) })
        }).catch(e => {
          wx.hideLoading()
          wx.showToast({ title: e.message || '上传失败', icon: 'none' })
        })
      },
      fail: () => {
        wx.hideLoading()
        wx.showToast({ title: '读图失败', icon: 'none' })
      }
    })
  },

  removePhoto(e) {
    const i = Number(e.currentTarget.dataset.i)
    const photos = this.data.photos.slice()
    photos.splice(i, 1)
    this.setData({ photos })
  },

  previewPhoto(e) {
    const i = Number(e.currentTarget.dataset.i)
    wx.previewImage({ current: this.data.photos[i], urls: this.data.photos })
  },

  /** 真实定位 */
  getLocation() {
    wx.getLocation({
      type: 'gcj02',
      success: res => {
        this.setData({ loc: res.latitude.toFixed(5) + ', ' + res.longitude.toFixed(5) })
        wx.showToast({ title: '已获取位置', icon: 'success' })
      },
      fail: () => {
        wx.showToast({ title: '未授权定位，可手填位置', icon: 'none' })
      }
    })
  },

  onLoc(e) {
    this.setData({ loc: e.detail.value })
  },

  submit() {
    const desc = (this.data.desc || '').trim()
    if (desc.length < 4 && !this.data.audio) {
      return wx.showToast({ title: '写一句或录一段语音', icon: 'none' })
    }
    if (this.data.submitting) return
    this.setData({ submitting: true })
    api.createIssue({
      type: this.data.types[this.data.type],
      title: desc.slice(0, 40) || ('语音上报 ' + this.data.audioSec + ' 秒'),
      desc,
      urgent: this.data.urgent === 1,
      loc: this.data.loc,
      photos: this.data.photos,
      audio: this.data.audio,
      audioSec: this.data.audioSec
    }).then(res => {
      this.setData({ submitting: false, desc: '', urgent: 0, photos: [], loc: '', audio: '', audioSec: 0 })
      wx.showModal({
        title: '已提交',
        content: '工单编号 ' + res.item.id + '\n安全科在后台能看到这条上报',
        showCancel: false
      })
      this.fetchMine()
    }).catch(e => {
      this.setData({ submitting: false })
      wx.showToast({ title: e.message, icon: 'none' })
    })
  },

  openDetail(e) {
    const it = this.data.items[Number(e.currentTarget.dataset.i)]
    if (!it) return
    wx.showModal({
      title: it.title,
      content: '类型：' + (it.type || '—') + '\n状态：' + (it.status_text || it.status) +
        '\n上报：' + (it.createdAt || '') +
        (it.handleNote ? '\n处置：' + it.handleNote : '\n处置：等待安全科处理'),
      showCancel: false
    })
  }
})
