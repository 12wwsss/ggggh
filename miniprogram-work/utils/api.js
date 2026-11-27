const { baseUrl, timeout } = require('./config.js')
const auth = require('./auth.js')

// 服务器地址：默认取 config.js，真机调试时可在登录页临时改（存本地，不重新编译）
const BASE_KEY = 'base_url'
function serverUrl() {
  const custom = wx.getStorageSync(BASE_KEY)
  return String(custom || baseUrl || '').replace(/\/+$/, '')
}
function setServerUrl(url) {
  const v = String(url || '').trim().replace(/\/+$/, '')
  if (v) wx.setStorageSync(BASE_KEY, v)
  else wx.removeStorageSync(BASE_KEY)
  return serverUrl()
}

function request(path, data, method = 'POST') {
  return new Promise((resolve, reject) => {
    const tk = auth.token ? auth.token() : ''
    wx.request({
      url: serverUrl() + path,
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
          : '连不上 ' + serverUrl() + '，检查手机和服务器是否在同一网络'
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

/** 师傅/培训员写带培小结（月度小结或期满总结） */
function saveTrainingNote(payload) {
  return request('/api/training/note', payload)
}

/** 师傅/培训员确认或撤销某个带培阶段 */
function toggleTrainingStage(payload) {
  return request('/api/training/stage', payload)
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

function issueResubmit(payload) {
  return request('/api/issue/resubmit', payload)
}

function notifications() {
  return request('/api/notifications', {}, 'GET')
}

function messages(query) {
  return request('/api/messages' + (query || ''), {}, 'GET')
}

function messageRead(id) {
  return request('/api/messages/read', { id })
}

function messageReadAll() {
  return request('/api/messages/read', { all: true })
}

function users(deptId) {
  const q = deptId ? '?dept_id=' + encodeURIComponent(deptId) : ''
  return request('/api/users' + q, {}, 'GET')
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


function learnItems() {
  return request('/api/learn/items', {}, 'GET')
}

function learnItemDetail(id) {
  return request('/api/learn/items/' + encodeURIComponent(id), {}, 'GET')
}

function learnProgress(id, progress, durationSec) {
  return request('/api/learn/items/' + encodeURIComponent(id) + '/progress', {
    progress,
    duration_sec: durationSec
  })
}

function noticeAck(id) {
  return request('/api/notice/read', { id })
}

function wrongList() {
  return request('/api/wrongbook', {}, 'GET')
}

function wrongSet(id, ok) {
  return request('/api/wrongbook', { id, ok }).catch(() => null)
}

function examRecords(paperId) {
  return request('/api/exam/records?paperId=' + encodeURIComponent(paperId), {}, 'GET')
}

function examReview(id) {
  return request('/api/exam/records/' + encodeURIComponent(id), {}, 'GET')
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

/**
 * 文化流：图文（/api/content，案例/安全月/知识卡）+ 文化（/api/culture，寄语/光荣榜/知识卡）合并。
 * 与 H5 同一口径：首页、文化列表、详情都用这一份数据，按日期倒序，按分类过滤。
 */
function cultureFeed(cat) {
  const q = cat ? '?cat=' + encodeURIComponent(cat) : ''
  return Promise.all([
    request('/api/content' + q, {}, 'GET').catch(() => ({ items: [] })),
    request('/api/culture', {}, 'GET').catch(() => ({ items: [] }))
  ]).then(([a, c]) => {
    const seen = {}
    const out = []
    ;(a.items || []).concat(c.items || []).forEach(it => {
      if (!it || !it.id || seen[it.id]) return
      seen[it.id] = 1
      if (cat && it.cat !== cat) return
      out.push(it)
    })
    out.sort((x, y) => String(y.date || '').localeCompare(String(x.date || '')))
    return { items: out }
  })
}

function wxLogin(code) {
  return request('/api/wx/login', { code })
}

/** 工号登录 */
function login(username, password) {
  return request('/api/login', { username, password })
}

/** 首次登录修改初始口令 */
function firstPassword(password) {
  return request('/api/password/first', { password })
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
  wxLogin, login, firstPassword, me, media, absUrl, options, questionGroups, practiceGroups, jobQuestions, employeeSummary, learnRead, risks, learnItems, learnItemDetail, learnProgress, wrongList, wrongSet, examRecords, examReview, faq, examList, examStart, examSubmit,
  culture, cultureFeed, profile, saveProfile,
  chat, submitQuiz, health,
  content, article, training, saveTrainingNote, toggleTrainingStage, serverUrl, setServerUrl,
  createIssue, issues, handleIssue, issueFlow, issueRectify, issueResubmit, notifications, messages, messageRead, messageReadAll, users, issueStats, upload,
  notices, noticeRead, noticeAck, quizStats, questions,
}
