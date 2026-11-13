import axios from 'axios'

const TOKEN_KEY = 'coalrag.token'
const USER_KEY = 'coalrag.user'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function getUser() {
  try {
    return JSON.parse(localStorage.getItem(USER_KEY) || 'null')
  } catch {
    return null
  }
}

export function saveSession(token, user) {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(USER_KEY, JSON.stringify(user || null))
}

export function clearSession() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

let unauthorizedHandler = null
export function onUnauthorized(fn) {
  unauthorizedHandler = fn
}

const http = axios.create({ timeout: 120000 })

http.interceptors.request.use((cfg) => {
  const token = getToken()
  if (token) cfg.headers.Authorization = `Bearer ${token}`
  return cfg
})

http.interceptors.response.use(
  (resp) => resp.data,
  (err) => {
    const status = err.response?.status
    const body = err.response?.data
    if (status === 401) {
      clearSession()
      if (unauthorizedHandler) unauthorizedHandler()
    }
    const message =
      body?.message ||
      (err.code === 'ECONNABORTED' ? '请求超时，请稍后重试' : '网络异常，请检查与服务器的连接')
    return Promise.reject(Object.assign(new Error(message), { status, body }))
  },
)

// 统一解开 { code, message, data } 信封；分页接口原样返回 { list, total }。
export async function request(method, url, data, config = {}) {
  const resp = await http.request({ method, url, data, ...config })
  if (resp && typeof resp === 'object' && 'data' in resp && 'code' in resp) {
    if (resp.code !== 0) throw new Error(resp.message || '请求失败')
    return resp.data
  }
  return resp
}

export const api = {
  get: (url, params) => request('get', url, undefined, { params }),
  post: (url, data) => request('post', url, data),
  put: (url, data) => request('put', url, data),
  del: (url) => request('delete', url),
  upload: (url, file, fields = {}) => {
    const form = new FormData()
    form.append('file', file)
    Object.entries(fields).forEach(([k, v]) => {
      if (v !== undefined && v !== null) form.append(k, v)
    })
    return request('post', url, form, { headers: { 'Content-Type': 'multipart/form-data' } })
  },
}

export async function login(username, password) {
  const data = await api.post('/api/v1/auth/login', { username, password })
  saveSession(data.token, data.user)
  return data
}

// 纯文本接口（如 /chunks/:id/text），不走 JSON 信封。
export async function fetchText(path) {
  const resp = await http.request({ method: 'get', url: path, responseType: 'text' })
  return typeof resp === 'string' ? resp : String(resp ?? '')
}

// 下载 CSV 等文件：带令牌取 blob，再触发浏览器保存。
export async function download(path, filename) {
  const blob = await http.request({ method: 'get', url: path, responseType: 'blob' })
  const url = URL.createObjectURL(blob instanceof Blob ? blob : new Blob([blob]))
  const a = document.createElement('a')
  a.href = url
  a.download = filename || path.split('/').pop() || 'download'
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}
