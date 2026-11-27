const api = require('../../utils/api.js')

Page({
  data: {
    loading: true,
    err: '',
    result: null,
    items: []
  },

  onLoad(options) {
    const id = Number(options && options.id)
    if (!id) {
      this.setData({ loading: false, err: '缺少答卷 id' })
      return
    }
    this.load(id)
  },

  load(id) {
    this.setData({ loading: true, err: '' })
    api.examReview(id).then(res => {
      const result = res.result || {}
      const items = (res.detail || []).map((d, qi) => {
        const opts = (d.o || []).map((text, oi) => {
          const key = 'ABCDEF'[oi]
          let cls = ''
          if (oi === d.a) cls = 'correct'
          else if (d.your && key === String(d.your).toUpperCase()) cls = 'wrong'
          return { key, text, cls, mark: oi === d.a ? '✓' : '' }
        })
        return {
          no: qi + 1,
          stem: d.q,
          ok: !!d.ok,
          opts,
          your: d.picked || (d.your ? '选了 ' + d.your : '未答'),
          right: d.right || '',
          analysis: d.e || ''
        }
      })
      this.setData({ loading: false, result, items })
    }).catch(e => {
      this.setData({ loading: false, err: e.message || '加载失败' })
    })
  },

  back() {
    wx.navigateBack({ delta: 1 })
  }
})
