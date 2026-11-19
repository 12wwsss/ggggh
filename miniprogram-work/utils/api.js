const { baseUrl, timeout } = require('./config.js')
const auth = require('./auth.js')

function request(path, data, method = 'POST') {
  return new Promise((resolve, reject) => {
    const tk = auth.token ? auth.token() : ''
    wx.request({
      url: baseUrl + path,
      method: method,
      data: data || {},
      timeout: timeout || 40000,
      header: {
        'content-type': 'application/json',
        'X-Device-Id': auth.deviceId ? auth.deviceId() : '',
        'X-Worker-Token': tk,
        Authorization: tk ? 'Bearer ' + tk : ''
      },
      success(res) {
        const d = res.data || {}
        if (res.statusCode >= 200 && res.statusCode < 300 && d.ok) return resolve(d)
        // 登录失效：清掉本地令牌并回登录页（登录接口本身除外）
        if (res.statusCode === 401 && path !== '/api/login') {
          auth.clearLogin()
          wx.reLaunch({ url: '/pages/login/login' })
        }
        // 把后端的业务错误转成一句人话
        let msg = d.message || d.error || '服务异常'
        if (res.statusCode === 404) {
          msg = '后端版本过旧（缺少新接口），请重启后端服务后再试'
        } else if (d.error === 'NO_KEY') {
          msg = 'AI 还没配置密钥，请先在后端填 DeepSeek Key'
        } else if (res.statusCode === 409) {
          msg = d.error || '该身份已绑定其他手机，请联系安全科解绑'
        } else if (d.error === 'not found') {
          msg = '后端版本过旧，请重启服务'
        }
        reject(new Error(msg))
      },
      fail(err) {
        const msg = /timeout/i.test(err.errMsg || '')
          ? '网络超时，井下信号可能不好，稍后再试'
          : '连不上后端，检查手机和服务器是否在同一网络'
        reject(new Error(msg))
      }
    })
  })
}

/** 问 AI：question 必填，history 用于多轮上下文，image 是已上传的图片地址 */
function chat(question, history, image) {
  return request('/api/chat', { question, history: history || [], image: image || '' })
}

/** 提交答题成绩（失败不打扰用户） */
function submitQuiz(payload) {
  return request('/api/quiz', payload).catch(() => null)
}

function health() {
  return request('/api/health', {}, 'GET')
}

// ---------- 内容 / 风险点 / 带培 ----------

/** 安全文化列表：cat 为空表示全部 */
function content(cat) {
  const q = cat ? '?cat=' + encodeURIComponent(cat) : ''
  return request('/api/content' + q, {}, 'GET')
}

/** 内容详情（含正文段落） */
function article(id) {
  return request('/api/content/' + encodeURIComponent(id), {}, 'GET')
}

/** 岗位风险点 */
function risks(job) {
  const q = job ? '?job=' + encodeURIComponent(job) : ''
  return request('/api/risks' + q, {}, 'GET')
}

/** 新员工带培计划 */
function training() {
  return request('/api/training', {}, 'GET')
}

// ---------- 问题工单 ----------

function upload(dataUrl) {
  return request('/api/upload', { data: dataUrl })
}

function createIssue(payload) {
  return request('/api/issue', payload)
}

function issues(status) {
  const q = status ? '?status=' + encodeURIComponent(status) : ''
  return request('/api/issues' + q, {}, 'GET')
}

function handleIssue(payload) {
  return request('/api/issue/handle', payload)
}

function issueFlow(id) {
  return request('/api/issue/flow?id=' + encodeURIComponent(id), {}, 'GET')
}

function issueRectify(payload) {
  return request('/api/issue/rectify', payload)
}

function users() {
  return request('/api/users', {}, 'GET')
}

function issueStats() {
  return request('/api/issues/stats', {}, 'GET')
}

// ---------- 制度宣贯 ----------

function notices(job) {
  const q = job ? '?job=' + encodeURIComponent(job) : ''
  return request('/api/notices' + q, {}, 'GET')
}

function questions(count, job, cat) {
  let url = '/api/questions?job=' + encodeURIComponent(job || '') + '&count=' + (count || 5)
  if (cat) url += '&c=' + encodeURIComponent(cat)
  return request(url, {}, 'GET')
}

function quizStats(user) {
  const q = user ? '?user=' + encodeURIComponent(user) : ''
  return request('/api/quiz/stats' + q, {}, 'GET')
}

function noticeRead(payload) {
  return request('/api/notice/read', payload).catch(() => null)
}

function faq(limit) {
  return request('/api/faq' + (limit ? '?limit=' + limit : ''), {}, 'GET')
}

function examList() {
  return request('/api/exams', {}, 'GET')
}

function examStart(examId, job) {
  return request('/api/exam/start', { examId, job: job || '' })
}

function examSubmit(sessionId, answers) {
  return request('/api/exam/submit', { sessionId, answers })
}

function employeeSummary() {
  return request('/api/employee/summary', {}, 'GET')
}

function learnRead(id, minutes) {
  return request('/api/learn/read', { id, minutes }).catch(() => null)
}

function questionGroups(job) {
  return request('/api/qgroups' + (job ? '?job=' + encodeURIComponent(job) : ''), {}, 'GET')
}

/** 按岗位练题的分组：通用 + 后台自定义岗位 */
function practiceGroups() {
  return request('/api/qgroups?by=job', {}, 'GET')
}

/** 某个岗位的全部题目（学习页浏览用，不随机） */
function jobQuestions(job) {
  return request('/api/questions?all=1&job=' + encodeURIComponent(job || ''), {}, 'GET')
}

function absUrl(u) {
  if (!u) return u
  if (/^https?:\/\//.test(u)) return u
  return baseUrl + (u.charAt(0) === '/' ? u : '/' + u)
}

function media(query) {
  return request('/api/materials' + (query || ''), {}, 'GET').then(res => {
    (res.items || []).forEach(x => { x.url = absUrl(x.url) })
    return res
  })
}

/** 制度宣贯 / 企业文化（后台可编辑） */
function culture() {
  return request('/api/culture', {}, 'GET')
}

function wxLogin(code) {
  return request('/api/wx/login', { code })
}

/** 工号登录 */
function login(username, password) {
  return request('/api/login', { username, password })
}

/** 当前登录人 */
function me() {
  return request('/api/me', {}, 'GET')
}

function options() {
  return request('/api/options', {}, 'GET')
}

/** 个人资料：头像 / 昵称 */
function profile() {
  return request('/api/profile', {}, 'GET')
}

function saveProfile(payload) {
  return request('/api/profile', payload)
}

module.exports = {
  wxLogin, login, me, media, absUrl, options, questionGroups, practiceGroups, jobQuestions, employeeSummary, learnRead, faq, examList, examStart, examSubmit,
  culture, profile, saveProfile,
  chat, submitQuiz, health,
  content, article, risks, training,
  createIssue, issues, handleIssue, issueFlow, issueRectify, users, issueStats, upload,
  notices, noticeRead, quizStats, questions,
}
