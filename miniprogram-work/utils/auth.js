// 身份：优先微信静默登录令牌，退化为本机设备号
const DEV_KEY = 'device_id'
const TOKEN_KEY = 'wx_token'
const USER_KEY = 'wx_user'

function deviceId() {
  let d = wx.getStorageSync(DEV_KEY)
  if (!d) {
    d = 'd' + Date.now().toString(36) + Math.random().toString(36).slice(2, 8)
    wx.setStorageSync(DEV_KEY, d)
  }
  return d
}

function token() { return wx.getStorageSync(TOKEN_KEY) || '' }
function setToken(t) { wx.setStorageSync(TOKEN_KEY, t) }
function user() { return wx.getStorageSync(USER_KEY) || null }
function setUser(u) { wx.setStorageSync(USER_KEY, u) }
function clearLogin() {
  wx.removeStorageSync(TOKEN_KEY)
  wx.removeStorageSync(USER_KEY)
}

module.exports = { deviceId, token, setToken, user, setUser, clearLogin }
