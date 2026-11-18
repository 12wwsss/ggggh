/**
 * 煤矿安全员 AI 智能体 · 后端代理（零依赖，Node 18+）
 *
 * 作用：
 *   1. 持有 DeepSeek 密钥（小程序里绝不能出现 key）
 *   2. 检索本地知识库，把相关条款塞进提示词（RAG-lite）
 *   3. 记录问答日志、答题成绩，给管理端出统计
 *   4. 限流，防止有人刷爆 token
 *
 * 启动：node index.js
 * 配置：同目录 config.json（参考 config.example.json），或环境变量 DEEPSEEK_API_KEY
 */
const http = require('http')
const https = require('https')
const fs = require('fs')
const path = require('path')

const ROOT = __dirname
const { makeStore } = require('./store.js')
const PORT = Number(process.env.PORT || 8789)
const BUILD_TAG = '2026-11-02-media'

// ---------------- 配置 ----------------
function loadConfig() {
  const cfg = {
    apiKey: process.env.DEEPSEEK_API_KEY || '',
    // ---- 部署相关：换环境只改这一段 ----
    deployMode: 'cloud',            // cloud 公网云 / lan 矿内局域网
    baseUrl: 'api.deepseek.com',    // 私有化时改成内网模型地址，如 10.0.0.21:8000
    modelPath: '/chat/completions', // vLLM/Ollama 的 OpenAI 兼容接口也是这个路径
    storageDir: 'data',             // 问答日志、答题成绩的本地存储目录
    trustProxy: false,              // 走 Nginx 反代时打开，用于取真实 IP
    // ---- 模型参数 ----
    model: 'deepseek-chat',
    temperature: 0.2,
    maxTokens: 900,
    topK: 4,
    rateLimitPerMin: 20,
    // 管理后台口令：留空 = 不校验（仅本机开发）；部署到公网前必须设置
    adminUser: 'admin',
    adminPassword: '',
    totpSecret: '',
    appId: '',
    appSecret: ''
  }
  const p = path.join(ROOT, 'config.json')
  if (fs.existsSync(p)) {
    try {
      Object.assign(cfg, JSON.parse(fs.readFileSync(p, 'utf8')))
    } catch (e) {
      console.error('[config] config.json 解析失败：', e.message)
    }
  }
  if (process.env.DEEPSEEK_API_KEY) cfg.apiKey = process.env.DEEPSEEK_API_KEY
  return cfg
}
const CFG = loadConfig()

// ---------------- 知识库 ----------------
function loadKB() {
  const dir = path.join(ROOT, 'kb')
  const chunks = []
  if (!fs.existsSync(dir)) return chunks
  for (const f of fs.readdirSync(dir)) {
    if (!/\.(md|txt)$/i.test(f)) continue
    const raw = fs.readFileSync(path.join(dir, f), 'utf8')
    const title = f.replace(/\.(md|txt)$/i, '')
    raw.split(/\n\s*\n/).forEach((block, i) => {
      const text = block.trim()
      if (text.length < 20) return
      chunks.push({ id: `${title}#${i + 1}`, title, text })
    })
  }
  console.log(`[kb] 载入 ${chunks.length} 个知识块，来自 ${dir}`)
  return chunks
}
let KB = loadKB()

// 中文按 2 字切词 + IDF 加权做粗检索，够用且不引依赖
function grams(s) {
  const clean = s.replace(/[^\u4e00-\u9fa5a-zA-Z0-9]/g, '')
  const set = new Set()
  for (let i = 0; i < clean.length - 1; i++) set.add(clean.slice(i, i + 2))
  return set
}

// 词频统计：越常见的二字词权重越低（"安全""作业"这类几乎每段都有）
let DF = {}
function rebuildIndex() {
  DF = {}
  KB.forEach(c => grams(c.text).forEach(g => { DF[g] = (DF[g] || 0) + 1 }))
}
rebuildIndex()
const idf = g => Math.log(1 + KB.length / (DF[g] || 0.5))

function reloadKB() {
  KB = loadKB()
  rebuildIndex()
  return KB.length
}

function retrieve(query, topK) {
  const qg = grams(query)
  if (!qg.size || !KB.length) return []
  let total = 0
  qg.forEach(g => { total += idf(g) })
  if (!total) return []

  const scored = KB.map(c => {
    const cg = grams(c.text)
    let hit = 0
    qg.forEach(g => { if (cg.has(g)) hit += idf(g) })
    return { ...c, score: hit / total }
  }).sort((a, b) => b.score - a.score)

  const top = scored.length ? scored[0].score : 0
  if (top < 0.08) return []
  // 绝对 + 相对双阈值：只留和最佳命中同一量级的片段，避免无关条款污染提示词
  const floor = Math.max(0.08, top * 0.55)
  return scored.filter(c => c.score >= floor).slice(0, topK)
}

// ---------------- 管理后台鉴权 ----------------
// 后台接口（需登录）。注意：小程序要用的 /api/quiz/stats、/api/issues/stats 不能放进来
const ADMIN_PATHS = ['/admin', '/qbank', '/api/kb/', '/api/export/', '/api/stats',
  '/api/questions/', '/api/audit', '/api/2fa/', '/api/users', '/api/appusers', '/api/me', '/api/materials/upload', '/api/materials/save', '/api/materials/delete',
  '/api/exams/all', '/api/exams/save', '/api/exams/delete', '/api/exam/stats',
  '/api/notices/save', '/api/notices/delete', '/api/news/save', '/api/news/delete',
  '/api/culture/save', '/api/culture/delete',
  '/api/options/save', '/api/qgroups/admin']
// 注意：/api/questions（无斜杠）是小程序抽题接口，不鉴权

// 登录会话（内存 token，重启服务需重新登录）
const SESSIONS = new Map()  // token -> { exp, user, role, display }
const SESSION_TTL = 12 * 60 * 60 * 1000

function needAuth(pathname) {
  if (!CFG.adminPassword) return false
  return ADMIN_PATHS.some(p => pathname === p || pathname.startsWith(p))
}

function parseCookies(req) {
  const out = {}
  ;(req.headers.cookie || '').split(';').forEach(kv => {
    const i = kv.indexOf('=')
    if (i > 0) out[kv.slice(0, i).trim()] = kv.slice(i + 1).trim()
  })
  return out
}

function currentSession(req) {
  const t = parseCookies(req).coal_admin
  if (!t) return null
  const rec = SESSIONS.get(t)
  if (!rec) return null
  if (Date.now() > rec.exp) { SESSIONS.delete(t); return null }
  return rec
}

// 登录失败锁定（等保：身份鉴别失败处理）
const LOGIN_FAILS = new Map()
const MAX_FAIL = 5
const LOCK_MS = 10 * 60 * 1000

function failKey(req) {
  return (req.headers['x-forwarded-for'] || req.socket.remoteAddress || 'unknown').split(',')[0].trim()
}

function isLocked(req) {
  const k = failKey(req)
  const rec = LOGIN_FAILS.get(k)
  if (!rec) return 0
  if (rec.until && Date.now() < rec.until) return Math.ceil((rec.until - Date.now()) / 60000)
  return 0
}

function markFail(req) {
  const k = failKey(req)
  const rec = LOGIN_FAILS.get(k) || { count: 0, until: 0 }
  rec.count++
  if (rec.count >= MAX_FAIL) {
    rec.until = Date.now() + LOCK_MS
    rec.count = 0
  }
  LOGIN_FAILS.set(k, rec)
}

function markSuccess(req) {
  LOGIN_FAILS.delete(failKey(req))
}

function passStrong(p) {
  return typeof p === 'string' && p.length >= 8 && /[A-Za-z]/.test(p) && /\d/.test(p)
}

function newSession(user) {
  const t = crypto.randomBytes(18).toString('hex')
  SESSIONS.set(t, {
    exp: Date.now() + SESSION_TTL,
    user: user.username,
    role: user.role,
    display: user.display || user.username
  })
  return t
}

// ---------------- TOTP（双因素） ----------------
const B32 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'

function b32encode(buf) {
  let bits = 0, value = 0, out = ''
  for (const byte of buf) {
    value = (value << 8) | byte
    bits += 8
    while (bits >= 5) { out += B32[(value >>> (bits - 5)) & 31]; bits -= 5 }
  }
  if (bits > 0) out += B32[(value << (5 - bits)) & 31]
  return out
}

function b32decode(str) {
  const clean = String(str).toUpperCase().replace(/[^A-Z2-7]/g, '')
  let bits = 0, value = 0
  const out = []
  for (const ch of clean) {
    const idx = B32.indexOf(ch)
    if (idx < 0) continue
    value = (value << 5) | idx
    bits += 5
    if (bits >= 8) { out.push((value >>> (bits - 8)) & 255); bits -= 8 }
  }
  return Buffer.from(out)
}

function totpCode(secretB32, step = 30, offset = 0) {
  const key = b32decode(secretB32)
  if (!key.length) return ''
  const counter = Math.floor(Date.now() / 1000 / step) + offset
  const buf = Buffer.alloc(8)
  buf.writeUInt32BE(Math.floor(counter / 0x100000000), 0)
  buf.writeUInt32BE(counter >>> 0, 4)
  const h = crypto.createHmac('sha1', key).update(buf).digest()
  const o = h[h.length - 1] & 0x0f
  const num = ((h[o] & 0x7f) << 24) | (h[o + 1] << 16) | (h[o + 2] << 8) | h[o + 3]
  return String(num % 1000000).padStart(6, '0')
}

/** 允许前后各 1 个时间窗（时钟漂移） */
function verifyTotp(secret, code) {
  const c = String(code || '').replace(/\D/g, '')
  if (c.length !== 6) return false
  for (const off of [-1, 0, 1]) {
    if (totpCode(secret, 30, off) === c) return true
  }
  return false
}

function totpUri(secret, account) {
  const label = encodeURIComponent('CoalMineAI:' + account)
  return `otpauth://totp/${label}?secret=${secret}&issuer=CoalMineAI&digits=6&period=30`
}

// ---------------- 审计日志哈希链 ----------------
const crypto = require('crypto')
const CHAIN = new Map() // file -> 最后一条的 hash

function chainSeed(file) {
  if (CHAIN.has(file)) return CHAIN.get(file)
  let last = 'genesis'
  try {
    const p = path.join(ROOT, CFG.storageDir, 'logs', file)
    if (fs.existsSync(p)) {
      const lines = fs.readFileSync(p, 'utf8').trim().split('\n').filter(Boolean)
      if (lines.length) {
        const o = JSON.parse(lines[lines.length - 1])
        last = o.h || 'genesis'
      }
    }
  } catch (e) { /* ignore */ }
  CHAIN.set(file, last)
  return last
}

function chainAppend(file, payload) {
  const prev = chainSeed(file)
  const body = JSON.stringify(payload)
  const h = crypto.createHash('sha256').update(prev + '|' + body).digest('hex').slice(0, 16)
  CHAIN.set(file, h)
  return Object.assign({}, payload, { prev, h })
}

function verifyChain(file) {
  const p = path.join(ROOT, CFG.storageDir, 'logs', file)
  if (!fs.existsSync(p)) return { file, ok: true, lines: 0 }
  const lines = fs.readFileSync(p, 'utf8').trim().split('\n').filter(Boolean)
  let prev = 'genesis'
  let legacy = 0
  for (let i = 0; i < lines.length; i++) {
    let o
    try { o = JSON.parse(lines[i]) } catch (e) {
      return { file, ok: false, lines: lines.length, brokenAt: i + 1, reason: '内容无法解析' }
    }
    // 启用哈希链之前写的历史记录没有 h 字段，跳过但计数
    if (!o.h) { legacy++; continue }
    if (o.prev !== prev) {
      return { file, ok: false, lines: lines.length, brokenAt: i + 1, reason: '前后链接断开（有行被删除或插入）' }
    }
    const body = {}
    Object.keys(o).forEach(k => { if (k !== 'prev' && k !== 'h') body[k] = o[k] })
    const h = crypto.createHash('sha256').update(prev + '|' + JSON.stringify(body)).digest('hex').slice(0, 16)
    if (h !== o.h) {
      return { file, ok: false, lines: lines.length, brokenAt: i + 1, reason: '内容被修改' }
    }
    prev = o.h
  }
  return { file, ok: true, lines: lines.length, legacy }
}

function persistConfig(patch) {
  try {
    const p = path.join(ROOT, 'config.json')
    let cur = {}
    if (fs.existsSync(p)) cur = JSON.parse(fs.readFileSync(p, 'utf8'))
    Object.assign(cur, patch)
    fs.writeFileSync(p, JSON.stringify(cur, null, 2) + '\n', 'utf8') // 不带 BOM
    return true
  } catch (e) {
    console.error('[config] 写入失败', e.message)
    return false
  }
}

async function handleChangePassword(req, res) {
  const b = await body(req)
  const oldPass = String(b.old || '')
  const newPass = String(b.next || '')
  // 改的是"当前登录账号"的密码，写进 users.json；不再动 config.json 的遗留字段
  const sess = currentSession(req)
  if (!sess) return json(res, 401, { ok: false, error: '登录已过期，请重新登录后再改' })
  const list = loadUsers()
  const u = list.find(x => x.username === sess.user)
  if (!u) return json(res, 404, { ok: false, error: '账号不存在' })
  if (hashPass(oldPass, u.salt) !== u.hash) return json(res, 400, { ok: false, error: '原密码不对' })
  if (!passStrong(newPass)) return json(res, 400, { ok: false, error: '新密码至少 8 位，且要同时包含字母和数字' })
  if (newPass === oldPass) return json(res, 400, { ok: false, error: '新密码不能和原密码一样' })
  const salt = crypto.randomBytes(8).toString('hex')
  u.salt = salt
  u.hash = hashPass(newPass, salt)
  u.pwChangedAt = new Date().toISOString()
  saveUsers(list)
  // 兼容老部署：admin 兼容账号改名时，同步 config.json 里的初始密码种子
  if (u.username === (CFG.adminUser || 'admin')) {
    try { persistConfig({ adminPassword: newPass }); CFG.adminPassword = newPass } catch (e) {}
  }
  SESSIONS.clear()  // 所有会话失效，强制重新登录
  log('auth.jsonl', { action: 'change-password', ip: failKey(req), user: u.username })
  json(res, 200, { ok: true, relogin: true })
}

function serveHtml(res, file) {
  const p = path.join(ROOT, 'public', file)
  if (!fs.existsSync(p)) {
    res.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' })
    return res.end('页面缺少：public/' + file)
  }
  const html = fs.readFileSync(p)
  res.writeHead(200, {
    'Content-Type': 'text/html; charset=utf-8',
    'Content-Length': html.length,
    'Cache-Control': 'no-store'
  })
  res.end(html)
}

// ---------------- 内容库（可运营） ----------------
function loadJSON(rel, fallback) {
  try {
    const p = path.join(ROOT, rel)
    if (!fs.existsSync(p)) return fallback
    return JSON.parse(fs.readFileSync(p, 'utf8'))
  } catch (e) {
    console.error('[content] 读取失败', rel, e.message)
    return fallback
  }
}

const CONTENT_DIR = process.env.CONTENT_DIR || 'content'
let ARTICLES = loadJSON(path.join(CONTENT_DIR, 'articles.json'), { items: [] }).items || []
let RISKS = loadJSON(path.join(CONTENT_DIR, 'risks.json'), { items: [] }).items || []
let TRAINING = loadJSON(path.join(CONTENT_DIR, 'training.json'), { plans: [] }).plans || []
let NOTICES = loadJSON(path.join(CONTENT_DIR, 'notices.json'), { items: [] }).items || []
let CULTURE = loadJSON(path.join(CONTENT_DIR, 'culture.json'), { items: [] }).items || []
let QUESTIONS = loadJSON(path.join(CONTENT_DIR, 'questions.json'), { items: [] }).items || []
let EXAMS = loadJSON(path.join(CONTENT_DIR, 'exams.json'), { items: [] }).items || []
let OPTIONS = loadJSON(path.join(CONTENT_DIR, 'options.json'), { teams: [], jobs: [], mediaCats: [] })
let NEWS = loadJSON(path.join(CONTENT_DIR, 'news.json'), { items: [] }).items || []
const QUESTION_FILE = path.join(ROOT, CONTENT_DIR, 'questions.json')
console.log(`[content] 文章 ${ARTICLES.length} 篇 / 风险点 ${RISKS.length} 条 / 带培 ${TRAINING.length} 个 / 制度 ${NOTICES.length} 条`)

/** 热重载：运营改完 JSON 不用重启服务 */
/** 岗位自定义：录题/建考试时遇到新岗位就自动登记 */
function ensureJob(job) {
  const j = String(job || '').trim()
  if (!j) return
  if (!Array.isArray(OPTIONS.jobs)) OPTIONS.jobs = []
  if (OPTIONS.jobs.includes(j)) return
  OPTIONS.jobs.push(j)
  try {
    fs.writeFileSync(path.join(ROOT, CONTENT_DIR, 'options.json'),
      JSON.stringify(OPTIONS, null, 2), 'utf8')
    log('admin.jsonl', { action: 'job-add', job: j })
  } catch (e) { /* ignore */ }
}

/** 调微信接口 */
function wxRequest(pathAndQuery) {
  return new Promise((resolve, reject) => {
    const req = https.request({ hostname: 'api.weixin.qq.com', path: pathAndQuery, method: 'GET', timeout: 8000 }, res => {
      let buf = ''
      res.setEncoding('utf8')
      res.on('data', c => { buf += c })
      res.on('end', () => {
        try { resolve(JSON.parse(buf)) } catch (e) { reject(new Error('微信返回非 JSON')) }
      })
    })
    req.on('timeout', () => req.destroy(new Error('微信接口超时')))
    req.on('error', reject)
    req.end()
  })
}

function reloadContent() {
  ARTICLES = loadJSON(path.join(CONTENT_DIR, 'articles.json'), { items: [] }).items || []
  RISKS = loadJSON(path.join(CONTENT_DIR, 'risks.json'), { items: [] }).items || []
  TRAINING = loadJSON(path.join(CONTENT_DIR, 'training.json'), { plans: [] }).plans || []
  NOTICES = loadJSON(path.join(CONTENT_DIR, 'notices.json'), { items: [] }).items || []
  CULTURE = loadJSON(path.join(CONTENT_DIR, 'culture.json'), { items: [] }).items || []
  QUESTIONS = loadJSON(path.join(CONTENT_DIR, 'questions.json'), { items: [] }).items || []
  EXAMS = loadJSON(path.join(CONTENT_DIR, 'exams.json'), { items: [] }).items || []
  OPTIONS = loadJSON(path.join(CONTENT_DIR, 'options.json'), { teams: [], jobs: [], mediaCats: [] })
  NEWS = loadJSON(path.join(CONTENT_DIR, 'news.json'), { items: [] }).items || []
}

// ---------------- 提示词 ----------------
const SYSTEM = `你是矿上的 AI 助手"小安"，能回答任何问题，不限于安全话题。

【回答方式】
1. 用户问什么就答什么：规程制度、操作做法、设备原理、算数、写字、常识、闲聊都可以。
2. 如果【知识库片段】里有相关内容，以片段为准，并在句末标出来源（依据：xxx）。
3. 知识库没有的，用你自己的知识正常回答，不用声明"知识库没有"，也不用加任何前缀。
4. 涉及本矿的具体规定、编号、处罚条款，而片段里又没有时，说"这条以本矿规程为准，建议向当班安全员确认"，但可以给出行业通行做法。
5. 绝不编造具体的规程编号、文件号和处罚金额。

【表达】
- 第一句直接给结论或答案，别铺垫、别复述问题。
- 安全操作、规程、处置类问题：按「先做什么 → 具体步骤 → 注意什么」展开，一般 3–5 条，每条一句话说清；有数字、标准、时限就带上。
- 设备原理、制度解释类：先一句结论，再补两三句为什么、怎么用。
- 闲聊、寒暄、简单确认：两三句即可，不用硬凑。
- 步骤、清单类用 1. 2. 3. 分条；口语化、工人看得懂，不用"综上所述""值得注意的是"这类腔调。
- 涉及人身安全的紧急情况（瓦斯超限、透水征兆、顶板来压等），第一句先给最要紧的动作。
- 长度随问题而定：闲聊两三句；操作和处置类要讲全，别为了短而漏掉关键步骤。

【底线】不输出违法、暴力、色情、政治敏感内容；这类问题简短拒绝并转回正题。`

function buildMessages(question, history, cites, imageData) {
  const ctx = cites.length
    ? cites.map((c, i) => `【片段${i + 1}｜${c.title}】\n${c.text}`).join('\n\n')
    : '（本次没有检索到本矿知识库内容，按你自身的知识正常回答即可，不用声明没有依据）'
  const msgs = [{ role: 'system', content: SYSTEM + '\n\n【知识库片段】\n' + ctx }]
  const hist = (history || []).slice(-6).map(m => ({
    role: m.role === 'ai' ? 'assistant' : 'user',
    content: String(m.content || '').slice(0, 800)
  }))
  msgs.push(...hist)
  const q = String(question).slice(0, 1000)
  if (imageData) {
    msgs.push({
      role: 'user',
      content: [
        { type: 'text', text: q },
        { type: 'image_url', image_url: { url: imageData } }
      ]
    })
  } else {
    msgs.push({ role: 'user', content: q })
  }
  return msgs
}

// ---------------- DeepSeek 调用 ----------------
function callDeepSeek(messages) {
  const payload = JSON.stringify({
    model: CFG.model,
    messages,
    temperature: CFG.temperature,
    max_tokens: CFG.maxTokens,
    stream: false
  })

  return new Promise((resolve, reject) => {
    const req = https.request({
      hostname: CFG.baseUrl,
      path: CFG.modelPath,
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(payload),
        Authorization: 'Bearer ' + CFG.apiKey
      },
      timeout: 45000
    }, res => {
      let buf = ''
      res.setEncoding('utf8')
      res.on('data', c => { buf += c })
      res.on('end', () => {
        if (res.statusCode !== 200) {
          return reject(new Error(`上游 ${res.statusCode}: ${buf.slice(0, 300)}`))
        }
        try {
          const json = JSON.parse(buf)
          const text = json.choices?.[0]?.message?.content || ''
          resolve({ text, usage: json.usage || {} })
        } catch (e) {
          reject(new Error('上游返回非 JSON：' + buf.slice(0, 200)))
        }
      })
    })
    req.on('timeout', () => req.destroy(new Error('上游超时 45s')))
    req.on('error', reject)
    req.write(payload)
    req.end()
  })
}

// ---------------- 设备标识（不再收集姓名/手机号） ----------------
const LEARN_FILE = 'learn.json'

function deviceOf(req) {
  return String(req.headers['x-device-id'] || '').trim()
}

// ---------------- 视频 / 资料库 ----------------
const MEDIA_FILE = 'media.json'
const MEDIA_EXT = {
  '.mp4': 'video', '.mov': 'video', '.m4v': 'video', '.webm': 'video',
  '.pdf': 'doc', '.doc': 'doc', '.docx': 'doc', '.xls': 'doc', '.xlsx': 'doc',
  '.ppt': 'doc', '.pptx': 'doc', '.txt': 'doc', '.zip': 'doc',
  '.jpg': 'image', '.jpeg': 'image', '.png': 'image', '.webp': 'image'
}

function loadMedia() {
  const d = store.read(MEDIA_FILE, { items: [] })
  return Array.isArray(d.items) ? d.items : []
}
function saveMedia(items) { store.write(MEDIA_FILE, { items }) }

function mediaTypeOf(name) {
  return MEDIA_EXT[path.extname(String(name)).toLowerCase()] || ''
}

function handleMediaList(res, url) {
  const cat = url.searchParams.get('cat') || ''
  const job = url.searchParams.get('job') || ''
  let items = loadMedia()
  if (cat) items = items.filter(x => x.cat === cat)
  if (job) items = items.filter(x => !x.jobs || !x.jobs.length || x.jobs.includes(job))
  items = items.slice().sort((a, b) => String(b.at).localeCompare(String(a.at)))
  json(res, 200, { ok: true, total: items.length, items })
}

async function handleMediaSave(req, res) {
  const b = await body(req)
  const item = b.item || {}
  const url = String(item.url || '').trim()
  if (!url) return json(res, 400, { ok: false, error: '缺少文件地址' })
  const rec = {
    id: String(item.id || ('m' + Date.now())),
    title: String(item.title || '').trim() || '未命名资料',
    cat: String(item.cat || '操作视频').trim(),
    type: String(item.type || mediaTypeOf(url) || 'doc'),
    url,
    size: Number(item.size) || 0,
    desc: String(item.desc || '').trim(),
    jobs: Array.isArray(item.jobs) ? item.jobs : (item.job ? [item.job] : []),
    at: new Date().toISOString()
  }
  const items = loadMedia().filter(x => x.id !== rec.id)
  items.unshift(rec)
  saveMedia(items)
  log('admin.jsonl', { action: 'media-save', id: rec.id, title: rec.title })
  json(res, 200, { ok: true, item: rec, total: items.length })
}

async function handleMediaDelete(req, res) {
  const b = await body(req)
  const id = String(b.id || '')
  const items = loadMedia()
  const target = items.find(x => x.id === id)
  const rest = items.filter(x => x.id !== id)
  saveMedia(rest)
  // 顺带删掉物理文件
  if (target && target.url && target.url.startsWith('/media/')) {
    try {
      const fp = path.join(MEDIA_DIR, path.basename(target.url))
      if (fs.existsSync(fp)) fs.unlinkSync(fp)
    } catch (e) { /* ignore */ }
  }
  log('admin.jsonl', { action: 'media-delete', id })
  json(res, 200, { ok: true, total: rest.length })
}

// ---------------- 小程序用户（微信/本机） ----------------
const APPUSERS_FILE = 'appusers.json'

function loadAppUsers() {
  const d = store.read(APPUSERS_FILE, { items: [] })
  return Array.isArray(d.items) ? d.items : []
}
function saveAppUsers(items) { store.write(APPUSERS_FILE, { items: items.slice(-2000) }) }

/** 记录/更新一个小程序用户 */
function touchAppUser(id, kind, label) {
  if (!id) return
  const items = loadAppUsers()
  const now = new Date().toISOString()
  let u = items.find(x => x.id === id)
  if (u) {
    u.lastAt = now
    u.logins = (u.logins || 1) + 1
    if (label) u.label = label
  } else {
    items.push({ id, kind, label: label || id.slice(-4), firstAt: now, lastAt: now, logins: 1 })
  }
  saveAppUsers(items)
}

/** 微信配置热加载：改了 config.json 不必重启也能生效 */
function refreshWxConfig() {
  if (CFG.appId && CFG.appSecret) return
  try {
    const p = path.join(ROOT, 'config.json')
    if (!fs.existsSync(p)) return
    const c = JSON.parse(fs.readFileSync(p, 'utf8').replace(/^\uFEFF/, ''))
    if (c.appId) CFG.appId = c.appId
    if (c.appSecret) CFG.appSecret = c.appSecret
  } catch (e) { /* ignore */ }
}

// 微信登录令牌 -> openid
const WX_SESSIONS = new Map()   // token -> { openid, at }

function wxOpenidOf(req) {
  const t = String(req.headers['x-worker-token'] || '').trim()
  if (!t) return ''
  const rec = WX_SESSIONS.get(t)
  return rec ? rec.openid : ''
}

/** 统一身份：微信 openid 优先，没有就用本机设备号 */
function identOf(req) {
  const openid = wxOpenidOf(req)
  if (openid) {
    return { key: 'wx:' + openid, label: '微信用户' + openid.slice(-4), kind: 'wx', openid }
  }
  const dev = deviceOf(req)
  if (dev) {
    return { key: 'dev:' + dev, label: '设备' + dev.slice(-4), kind: 'dev', device: dev }
  }
  return null
}

/** 兼容旧调用：交卷/上报需要一个显示名 */
function workerOf(req) {
  const id = identOf(req)
  if (!id) return null
  try { touchAppUser(id.key, id.kind, id.label) } catch (e) {}
  return { name: id.label, device: id.device || id.openid, kind: id.kind }
}

function loadLearn() { return store.read(LEARN_FILE, {}) }
function saveLearn(o) { store.write(LEARN_FILE, o) }

function learnOf(key) {
  const all = loadLearn()
  return all[key] || { reads: [], minutes: 0, lastAt: null }
}

async function handleLearnRead(req, res) {
  const b = await body(req)
  const key = deviceOf(req) || 'global'
  const id = String(b.id || '').trim()
  const minutes = Math.max(1, Math.min(30, Number(b.minutes) || 3))
  if (!id) return json(res, 400, { ok: false, error: '缺少内容 id' })

  const all = loadLearn()
  const rec = all[key] || { reads: [], minutes: 0, lastAt: null }
  if (!Array.isArray(rec.reads)) rec.reads = []
  if (!rec.reads.includes(id)) rec.reads.push(id)
  rec.minutes = (rec.minutes || 0) + minutes
  rec.lastAt = new Date().toISOString()
  all[key] = rec
  saveLearn(all)
  json(res, 200, { ok: true, readCount: rec.reads.length, minutes: rec.minutes })
}

function pctOf(r) { return r.total ? Math.round((r.score / r.total) * 100) : 0 }

function handleEmployeeSummary(res, url, req) {
  const key = deviceOf(req) || url.searchParams.get('device') || 'global'
  const label = key === 'global' ? '未标识' : '设备' + key.slice(-4)

  const learn = learnOf(key)
  const quiz = store.read('quiz-records.json', []).filter(r => r.user === label)
  const issues = store.read('issues.json', []).filter(i => (i.reporter || '') === label)
  const pending = issues.filter(i => i.status !== '已闭环').length

  const best = quiz.reduce((m, r) => Math.max(m, pctOf(r)), 0)
  const pass = quiz.filter(r => pctOf(r) >= 60).length
  const minutes = learn.minutes || 0
  const hours = Math.round((minutes / 60) * 10) / 10

  const risks = RISKS
  const readSet = new Set(learn.reads || [])
  const riskRead = risks.filter(r => readSet.has('risk:' + r.id)).length

  json(res, 200, {
    ok: true,
    label,
    hours,
    minutes,
    readCount: (learn.reads || []).length,
    quiz: {
      attempts: quiz.length,
      best,
      avg: quiz.length ? Math.round(quiz.reduce((a, r) => a + pctOf(r), 0) / quiz.length) : 0,
      passRate: quiz.length ? Math.round((pass / quiz.length) * 100) : 0
    },
    issue: { total: issues.length, pending },
    risk: { total: risks.length, read: riskRead, progress: risks.length ? Math.round((riskRead / risks.length) * 100) : 0 }
  })
}

// ---------------- 题库 ----------------
// 题库文件变化时自动重载（题库维护端在另一个端口写文件）
let watchTimer = null
try {
  fs.watch(QUESTION_FILE, { persistent: false }, () => {
    clearTimeout(watchTimer)
    watchTimer = setTimeout(() => {
      try {
        QUESTIONS = loadJSON(path.join(CONTENT_DIR, 'questions.json'), { items: [] }).items || []
        console.log(`[题库] 已热更新：${QUESTIONS.length} 道`)
      } catch (e) { /* ignore */ }
    }, 400)
  })
} catch (e) {
  console.warn('[题库] 监听文件失败，仍需手动重启：', e.message)
}

function saveQuestions() {
  fs.writeFileSync(QUESTION_FILE, JSON.stringify({
    note: '题库：可在管理后台增删改；on=false 表示停用（不参与抽题，但保留历史）',
    items: QUESTIONS
  }, null, 2), 'utf8')
}

function shuffle(arr) {
  const a = arr.slice()
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[a[i], a[j]] = [a[j], a[i]]
  }
  return a
}

/** 抽题：按岗位优先，不够用通用题补齐 */
function pickQuestions(job, count, cat) {
  const n = Math.min(Math.max(Number(count) || 5, 1), 50)
  const live = QUESTIONS.filter(q => q.on !== false)
  const jobOf = q => String(q.job || '').trim()
  const knownJobs = new Set((OPTIONS.jobs || []).map(x => String(x).trim()).filter(Boolean))
  const isGeneral = q => {
    const j = jobOf(q)
    return !j || j === '通用' || !knownJobs.has(j)
  }
  // 指定岗位时只取该岗位题库，绝不混入其他岗位；job=通用 只取未分配/通用题
  let pool = job
    ? (job === '通用'
        ? live.filter(isGeneral)
        : live.filter(q => jobOf(q) === job || isGeneral(q)))
    : live
  if (cat) {
    const byCat = pool.filter(q => String(q.c || '').trim() === cat)
    if (byCat.length) pool = byCat
  }
  return shuffle(pool).slice(0, n)
}

function handleQuestions(res, url) {
  const job = url.searchParams.get('job') || ''
  const cat = url.searchParams.get('c') || ''
  const count = Number(url.searchParams.get('count') || 5)
  // all=1：按岗位取全部题目（学习页浏览用，不随机、不截断）
  if (url.searchParams.get('all') === '1') {
    const live = QUESTIONS.filter(q => q.on !== false)
    const jobOf = q => String(q.job || '').trim()
    const known = new Set((OPTIONS.jobs || []).map(x => String(x).trim()).filter(Boolean))
    const isGeneral = q => {
      const j = jobOf(q)
      return !j || j === '通用' || !known.has(j)
    }
    let pool = job
      ? (job === '通用' ? live.filter(isGeneral) : live.filter(q => jobOf(q) === job))
      : live
    if (cat) pool = pool.filter(q => String(q.c || '').trim() === cat)
    const items = pool.map(q => ({ id: q.id, job: q.job, q: q.q, o: q.o, a: q.a, e: q.e, c: q.c }))
    return json(res, 200, { ok: true, total: items.length, items })
  }
  const items = pickQuestions(job, count, cat).map(q => ({
    id: q.id, job: q.job, q: q.q, o: q.o, a: q.a, e: q.e, c: q.c
  }))
  json(res, 200, { ok: true, total: QUESTIONS.length, items })
}

function handleQuestionsAdmin(res, url) {
  const job = url.searchParams.get('job') || ''
  const kw = (url.searchParams.get('kw') || '').trim()
  let items = QUESTIONS
  if (job) items = items.filter(q => q.job === job)
  if (kw) items = items.filter(q => (q.q + q.e + q.c).includes(kw))
  const byJob = {}
  QUESTIONS.forEach(q => {
    const k = String(q.job || '').trim() || '未分配岗位'
    byJob[k] = (byJob[k] || 0) + 1
  })
  json(res, 200, {
    ok: true,
    total: QUESTIONS.length,
    live: QUESTIONS.filter(q => q.on !== false).length,
    byJob,
    items
  })
}

async function handleQuestionSave(req, res) {
  const b = await body(req)
  const q = b.item || {}
  if (!q.q || String(q.q).trim().length < 5) return json(res, 400, { ok: false, error: '题干太短' })
  if (!Array.isArray(q.o) || q.o.length < 2) return json(res, 400, { ok: false, error: '至少两个选项' })
  if (!(q.a >= 0 && q.a < q.o.length)) return json(res, 400, { ok: false, error: '正确答案下标不合法' })

  const item = {
    id: q.id || 'q' + Date.now(),
    job: String(q.job == null ? '' : q.job).trim(),
    q: String(q.q).trim(),
    o: q.o.map(x => String(x).trim()),
    a: Number(q.a),
    e: String(q.e || '').trim(),
    c: String(q.c || '').trim(),
    on: q.on !== false
  }
  const i = QUESTIONS.findIndex(x => x.id === item.id)
  if (i >= 0) QUESTIONS[i] = item
  else QUESTIONS.push(item)
  saveQuestions()
  log('question.jsonl', { action: i >= 0 ? 'update' : 'create', id: item.id })
  json(res, 200, { ok: true, item, total: QUESTIONS.length })
}

async function handleQuestionDelete(req, res) {
  const b = await body(req)
  const i = QUESTIONS.findIndex(x => x.id === b.id)
  if (i < 0) return json(res, 404, { ok: false, error: '题目不存在' })
  QUESTIONS.splice(i, 1)
  saveQuestions()
  log('question.jsonl', { action: 'delete', id: b.id })
  json(res, 200, { ok: true, total: QUESTIONS.length })
}

async function handleQuestionToggle(req, res) {
  const b = await body(req)
  const q = QUESTIONS.find(x => x.id === b.id)
  if (!q) return json(res, 404, { ok: false, error: '题目不存在' })
  q.on = !(q.on !== false)
  saveQuestions()
  json(res, 200, { ok: true, item: q })
}

function parseQuestionTable(text) {
  const rows = String(text).split(/\r?\n/).map(l => l.trim()).filter(Boolean)
  const out = [], errs = []
  rows.forEach((line, idx) => {
    if (/^岗位\s*[,\t]/.test(line)) return
    const cells = line.split(/\t|,(?=(?:[^"]*"[^"]*")*[^"]*$)/).map(c => c.replace(/^"|"$/g, '').trim())
    if (cells.length < 5) { errs.push('第 ' + (idx + 1) + ' 行列数不足'); return }
    const [job, q, a, bb, c, d, ans, exp, cite] = cells
    const opts = [a, bb, c, d].filter(x => x !== undefined && x !== '')
    const ai = 'ABCD'.indexOf(String(ans || 'A').trim().toUpperCase())
    if (!q || opts.length < 2) { errs.push('第 ' + (idx + 1) + ' 行题干或选项缺失'); return }
    if (ai < 0 || ai >= opts.length) { errs.push('第 ' + (idx + 1) + ' 行正确答案超出范围'); return }
    out.push({ job: job || '通用', q, o: opts, a: ai, e: exp || '', c: cite || '', on: true })
  })
  return { out, errs }
}

function questionCsv(res, template) {
  let rows
  if (template) {
    rows = [
      ['岗位', '题干', '选项A', '选项B', '选项C', '选项D', '正确答案', '解析', '依据'],
      ['采煤', '液压支架检修前第一步是什么？', '直接拆管', '卸载泄压并确认无压', '通知班长', '边用边修', 'B', '带压拆管会被高压液击伤', '设备检修'],
      ['通用', '入井前必须检查什么？', '手机电量', '矿灯、安全帽、自救器', '只要安全帽', '不需要', 'B', '三件套缺一不可', '入井管理']
    ]
  } else {
    rows = [['岗位', '题干', '选项A', '选项B', '选项C', '选项D', '正确答案', '解析', '依据', '状态']]
    QUESTIONS.forEach(q => rows.push([
      q.job, q.q, q.o[0] || '', q.o[1] || '', q.o[2] || '', q.o[3] || '',
      'ABCD'[q.a] || 'A', q.e || '', q.c || '', q.on === false ? '停用' : '启用'
    ]))
  }
  const csv = '\uFEFF' + rows.map(r => r.map(v => '"' + String(v).replace(/"/g, '""') + '"').join(',')).join('\r\n')
  const name = template ? '题库导入模板.csv' : '题库导出.csv'
  res.writeHead(200, {
    'Content-Type': 'text/csv; charset=utf-8',
    'Content-Disposition': `attachment; filename="q.csv"; filename*=UTF-8''${encodeURIComponent(name)}`,
    'Content-Length': Buffer.byteLength(csv)
  })
  res.end(csv)
}

async function handleQuestionImport(req, res) {
  const b = await body(req)
  let arr = b.items
  let parseErrs = []
  if (!Array.isArray(arr) && typeof b.text === 'string' && b.text.trim()) {
    const r = parseQuestionTable(b.text)
    arr = r.out
    parseErrs = r.errs
  }
  if (!Array.isArray(arr)) {
    try { arr = JSON.parse(String(b.raw || '')) } catch (e) {
      return json(res, 400, { ok: false, error: '格式不对：需要 JSON 数组，或从 Excel 粘贴的制表符文本' })
    }
  }
  if (!Array.isArray(arr)) return json(res, 400, { ok: false, error: '格式不对，需要 JSON 数组' })

  let added = 0
  const errs = parseErrs.slice()
  arr.forEach((q, idx) => {
    if (!q || !q.q || !Array.isArray(q.o) || q.o.length < 2 || !(q.a >= 0 && q.a < q.o.length)) {
      errs.push(`第 ${idx + 1} 条不合法`)
      return
    }
    QUESTIONS.push({
      id: q.id || 'q' + Date.now() + '_' + idx,
      job: q.job || '通用',
      q: String(q.q).trim(),
      o: q.o.map(x => String(x).trim()),
      a: Number(q.a),
      e: String(q.e || '').trim(),
      c: String(q.c || '').trim(),
      on: q.on !== false
    })
    added++
  })
  saveQuestions()
  log('question.jsonl', { action: 'import', added })
  json(res, 200, { ok: true, added, failed: errs.length, errors: errs.slice(0, 5), total: QUESTIONS.length })
}

// ---------------- 在线考试 ----------------
const EXAM_FILE = 'exam-records.json'
const EXAM_SESSIONS = new Map()   // id -> { device, examId, questions, deadline }
const EXAM_GRACE_MS = 30000       // 网络抖动宽限 30 秒

function examRecords() { return store.read(EXAM_FILE, []) }
function saveExamRecords(list) { store.write(EXAM_FILE, list) }

function myExamRecords(device) {
  return examRecords().filter(r => r.device === device)
}

function handleExamList(res, url, req) {
  const device = deviceOf(req) || url.searchParams.get('device') || ''
  const rows = myExamRecords(device)
  const items = EXAMS.map(e => {
    const done = rows.find(r => r.examId === e.id)
    return {
      id: e.id, title: e.title, job: e.job, questions: e.questions,
      minutes: e.minutes, pass: e.pass, note: e.note,
      status: done ? 'done' : 'idle',
      score: done ? done.score : null,
      total: done ? done.total : null,
      passed: done ? done.passed : null,
      submittedAt: done ? done.at : null
    }
  })
  json(res, 200, { ok: true, items })
}

async function handleExamStart(req, res) {
  const b = await body(req)
  const device = deviceOf(req) || String(b.device || '')
  const examId = String(b.examId || '')
  if (!device) return json(res, 400, { ok: false, error: '设备标识缺失' })
  const exam = EXAMS.find(e => e.id === examId)
  if (!exam) return json(res, 404, { ok: false, error: '考试不存在' })
  if (myExamRecords(device).some(r => r.examId === examId)) {
    return json(res, 409, { ok: false, error: '你已经交过这份卷，不能重考' })
  }

  // 岗位专用考试：声明岗位不符直接拒绝（成绩会计入档案，别替别人考）
  const declared = String(b.job || '').trim()
  if (exam.job && declared && declared !== exam.job) {
    return json(res, 403, { ok: false, error: `这场考试仅限「${exam.job}」岗位，你当前是「${declared}」` })
  }

  let pool = QUESTIONS.filter(q => q.on !== false)
  if (exam.job) {
    pool = pool.filter(q => {
      const j = String(q.job || '').trim()
      return j === exam.job || j === '通用'
    })
    if (pool.length < 1) {
      return json(res, 400, { ok: false, error: `「${exam.job}」岗位题库还没有题目，请先在后台录入` })
    }
  }
  if (pool.length < 2) return json(res, 400, { ok: false, error: '题库题目不足 2 道，请先在后台补充' })

  const shuffled = shuffle(pool).slice(0, Math.min(exam.questions, pool.length))
  const id = 'S' + Date.now().toString(36) + Math.random().toString(36).slice(2, 6)
  const deadline = Date.now() + exam.minutes * 60 * 1000

  EXAM_SESSIONS.set(id, { device, examId, questions: shuffled, deadline, startedAt: Date.now(), declared })

  json(res, 200, {
    ok: true,
    sessionId: id,
    minutes: exam.minutes,
    deadline,
    total: shuffled.length,
    questions: shuffled.map(q => ({ id: q.id, q: q.q, o: q.o }))
  })
}

async function handleExamSubmit(req, res) {
  const b = await body(req)
  const device = deviceOf(req) || String(b.device || '')
  const sid = String(b.sessionId || '')
  const sess = EXAM_SESSIONS.get(sid)
  if (!sess) return json(res, 404, { ok: false, error: '考试会话已失效，请重新开始' })
  if (sess.device !== device) return json(res, 403, { ok: false, error: '会话不属于本机' })

  const exam = EXAMS.find(e => e.id === sess.examId) || { pass: 60 }
  const answers = Array.isArray(b.answers) ? b.answers : []
  let score = 0
  const detail = sess.questions.map((q, i) => {
    const picked = Number(answers[i])
    const right = picked === q.a
    if (right) score++
    return {
      id: q.id, job: q.job || '', q: q.q, o: q.o, a: q.a,
      right: q.o[q.a], picked: (picked >= 0 && q.o[picked]) || '',
      ok: right, e: q.e || '', c: q.c || ''
    }
  })

  const total = sess.questions.length
  const pct = total ? Math.round((score / total) * 100) : 0
  const late = Date.now() > sess.deadline + EXAM_GRACE_MS

  const rec = {
    id: 'E' + Date.now(),
    device,
    user: (workerOf(req) && workerOf(req).name) || ('设备' + device.slice(-4)),
    examId: sess.examId,
    title: exam.title || '',
    job: exam.job || '',
    declaredJob: sess.declared || '',
    score: pct,
    raw: score,
    total,
    passed: pct >= (exam.pass || 60),
    late,
    at: new Date().toISOString(),
    usedSec: Math.round((Date.now() - sess.startedAt) / 1000)
  }
  const list = examRecords()
  list.unshift(rec)
  saveExamRecords(list.slice(0, 5000))
  EXAM_SESSIONS.delete(sid)
  log('exam.jsonl', { examId: rec.examId, user: rec.user, score: pct, passed: rec.passed, late })
  json(res, 200, { ok: true, result: rec, detail })
}

// ---------------- 管理后台接口 ----------------
function kbList() {
  const dir = path.join(ROOT, 'kb')
  if (!fs.existsSync(dir)) return []
  return fs.readdirSync(dir).filter(f => /\.(md|txt)$/i.test(f)).map(f => {
    const st = fs.statSync(path.join(dir, f))
    const chunks = KB.filter(c => c.title === f.replace(/\.(md|txt)$/i, '')).length
    return { name: f, size: st.size, updated: st.mtime.toISOString(), chunks }
  }).sort((a, b) => b.updated.localeCompare(a.updated))
}

const KB_PENDING = path.join(ROOT, 'kb_pending')
function ensurePending() { if (!fs.existsSync(KB_PENDING)) fs.mkdirSync(KB_PENDING, { recursive: true }) }

function listPending() {
  ensurePending()
  return fs.readdirSync(KB_PENDING).filter(f => /\.(md|txt)$/i.test(f)).map(f => {
    const st = fs.statSync(path.join(KB_PENDING, f))
    return { name: f, size: st.size, updated: st.mtime.toISOString() }
  }).sort((a, b) => b.updated.localeCompare(a.updated))
}

async function handleKbApprove(req, res) {
  const b = await body(req)
  const raw = String(b.name || '').trim()
  if (!raw || !/\.(md|txt)$/i.test(raw)) {
    return json(res, 400, { ok: false, error: '文件名无效' })
  }
  const name = path.basename(raw)
  const src = path.join(KB_PENDING, name)
  if (!fs.existsSync(src)) return json(res, 404, { ok: false, error: '待审文件不存在' })
  const dst = path.join(ROOT, 'kb', name)
  if (!fs.existsSync(path.join(ROOT, 'kb'))) fs.mkdirSync(path.join(ROOT, 'kb'), { recursive: true })
  if (b.action === 'reject') {
    fs.unlinkSync(src)
    log('kb.jsonl', { action: 'reject', file: name, by: 'admin' })
    return json(res, 200, { ok: true, rejected: true })
  }
  fs.renameSync(src, dst)
  const n = reloadKB()
  log('kb.jsonl', { action: 'approve', file: name, by: 'admin', chunks: n })
  json(res, 200, { ok: true, kbChunks: n })
}

async function handleKbUpload(req, res) {
  const b = await body(req)
  const name = String(b.name || '').trim()
  const content = String(b.content || '')
  if (!/\.(md|txt)$/i.test(name)) return json(res, 400, { ok: false, error: '只支持 .md / .txt，Word 请先另存为 txt' })
  if (content.trim().length < 20) return json(res, 400, { ok: false, error: '内容太短，至少 20 字' })
  if (content.length > 2 * 1024 * 1024) return json(res, 400, { ok: false, error: '单个文件不要超过 2MB' })

  const safe = name.replace(/[\\/:*?"<>|]/g, '_')
  ensurePending()
  fs.writeFileSync(path.join(KB_PENDING, safe), content, 'utf8')
  log('kb.jsonl', { action: 'upload-pending', file: safe, bytes: content.length, by: 'admin' })
  json(res, 200, {
    ok: true,
    file: safe,
    pending: true,
    message: '已提交，待矿上管理员审核通过后生效'
  })
}

async function handleKbDelete(req, res) {
  const b = await body(req)
  const name = path.basename(String(b.name || ''))
  const p = path.join(ROOT, 'kb', name)
  if (!fs.existsSync(p)) return json(res, 404, { ok: false, error: '文件不存在' })
  fs.unlinkSync(p)
  const n = reloadKB()
  log('kb.jsonl', { action: 'delete', file: name })
  json(res, 200, { ok: true, kbChunks: n })
}

function csvLine(arr) {
  return arr.map(v => '"' + String(v == null ? '' : v).replace(/"/g, '""') + '"').join(',')
}

function sendCsv(res, filename, rows) {
  const body = '\uFEFF' + rows.map(csvLine).join('\r\n')
  // HTTP 头只能是 ASCII，中文文件名走 RFC 5987
  res.writeHead(200, {
    'Content-Type': 'text/csv; charset=utf-8',
    'Content-Disposition': `attachment; filename="export.csv"; filename*=UTF-8''${encodeURIComponent(filename)}`,
    'Content-Length': Buffer.byteLength(body)
  })
  res.end(body)
}

function exportIssues(res) {
  const all = store.read('issues.json', [])
  const rows = [['工单号', '类型', '标题', '位置', '上报人', '区队', '紧急', '状态', '上报时间', '处置人', '处置说明']]
  all.forEach(i => rows.push([
    i.id, i.type, i.title, i.loc, i.reporter, i.team,
    i.urgent ? '紧急' : '一般', i.status, i.createdAt, i.handler, i.handleNote
  ]))
  sendCsv(res, '问题报表.csv', rows)
}

function exportQuiz(res) {
  const all = store.read('quiz-records.json', [])
  const rows = [['时间', '姓名', '区队', '岗位', '得分', '题数', '百分制', '用时(秒)']]
  all.forEach(r => rows.push([
    r.at, r.user, r.team, r.job, r.score, r.total,
    r.total ? Math.round((r.score / r.total) * 100) : 0, r.spent
  ]))
  sendCsv(res, '考试成绩.csv', rows)
}

function adminPage(res) {
  const p = path.join(ROOT, 'public', 'index.html')
  if (!fs.existsSync(p)) {
    res.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' })
    return res.end('管理页面未找到：server/public/index.html')
  }
  const html = fs.readFileSync(p)
  res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8', 'Content-Length': html.length })
  res.end(html)
}

// ---------------- 问题工单 ----------------
const store = makeStore(path.join(ROOT, CFG.storageDir))
const UPLOAD_DIR = path.join(ROOT, CFG.storageDir, 'uploads')
const MEDIA_DIR = path.join(ROOT, CFG.storageDir, 'media')
try { fs.mkdirSync(UPLOAD_DIR, { recursive: true }) } catch (e) {}
try { fs.mkdirSync(MEDIA_DIR, { recursive: true }) } catch (e) {}

// ---------------- 用户与角色（三员分立） ----------------
const USERS_FILE = 'users.json'
const ROLES = {
  sysadmin: '系统管理员',
  secadmin: '安全管理员',
  auditor: '审计管理员'
}

function hashPass(pw, salt) {
  return crypto.scryptSync(pw, salt, 64).toString('hex')
}

function newUser(username, pw, role, display) {
  const salt = crypto.randomBytes(8).toString('hex')
  return {
    username,
    display: display || ROLES[role] || username,
    role,
    salt,
    hash: hashPass(pw, salt),
    disabled: false,
    createdAt: new Date().toISOString(),
    lastLogin: null
  }
}

function loadUsers() {
  let list = store.read(USERS_FILE, null)
  if (!Array.isArray(list) || !list.length) {
    const seed = CFG.adminPassword || 'coal2026'
    list = [
      newUser('sysadmin', seed, 'sysadmin', '系统管理员'),
      newUser('secadmin', seed, 'secadmin', '安全管理员'),
      newUser('auditor', seed, 'auditor', '审计管理员')
    ]
    // 兼容老账号 admin：密码相同，按系统管理员处理
    list.push(newUser(CFG.adminUser || 'admin', seed, 'sysadmin', '系统管理员(兼容账号)'))
    store.write(USERS_FILE, list)
    console.log('[users] 已初始化三员账号，初始密码与 adminPassword 相同')
  }
  return list
}

function saveUsers(list) { store.write(USERS_FILE, list) }

function findUser(name) {
  return loadUsers().find(u => u.username === name) || null
}

function verifyUser(name, pw) {
  const u = findUser(name)
  if (!u || u.disabled) return null
  const h = hashPass(pw, u.salt)
  return h === u.hash ? u : null
}

// 角色可见的后台接口（未列出的管理接口默认全部放行）
const ROLE_RULES = [
  { prefix: '/api/users', roles: ['sysadmin'] },
  { prefix: '/api/kb/approve', roles: ['secadmin'] },
  { prefix: '/api/kb/', roles: ['sysadmin', 'secadmin'] },
  { prefix: '/api/questions/', roles: ['sysadmin', 'secadmin'] },
  { prefix: '/api/export/', roles: ['sysadmin', 'secadmin'] },
  { prefix: '/api/audit', roles: ['sysadmin', 'auditor'] },
  { prefix: '/api/issues', roles: ['sysadmin', 'secadmin'] }
]

function roleAllowed(role, pathname) {
  const rule = ROLE_RULES.find(r => pathname.startsWith(r.prefix))
  if (!rule) return true
  return rule.roles.includes(role)
}

function nextIssueId(list) {
  const d = new Date()
  const day = `${d.getFullYear()}${String(d.getMonth() + 1).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}`
  const today = list.filter(i => (i.id || '').includes(day)).length + 1
  return `I${day}-${String(today).padStart(3, '0')}`
}

async function handleIssueCreate(req, res) {
  const b = await body(req)
  const title = String(b.title || b.desc || '').trim()
  if (title.length < 4) return json(res, 400, { ok: false, error: '问题描述太短，写清楚位置和现象' })

  const list = store.read('issues.json', [])
  const item = {
    id: nextIssueId(list),
    type: b.type || '安全隐患',
    title: title.slice(0, 80),
    desc: String(b.desc || '').slice(0, 500),
    urgent: !!b.urgent,
    loc: b.loc || '',
    photos: Array.isArray(b.photos) ? b.photos.slice(0, 3) : [],
    audio: String(b.audio || '').slice(0, 300),
    audioSec: Number(b.audioSec) || 0,
    reporter: (workerOf(req) && workerOf(req).name) || (workerOf(req) && workerOf(req).phoneMask) || b.reporter || '未绑定',
    team: (workerOf(req) && workerOf(req).team) || b.team || '未登记',
    status: '待分派',
    createdAt: new Date().toISOString(),
    handledAt: null,
    handleNote: '',
    handler: ''
  }
  list.unshift(item)
  store.write('issues.json', list.slice(0, 500))
  log('issue.jsonl', { id: item.id, type: item.type, title: item.title, urgent: item.urgent })
  json(res, 200, { ok: true, item })
}

function handleIssueList(res, url) {
  const status = url.searchParams.get('status') || ''
  const all = store.read('issues.json', [])
  const list = status ? all.filter(i => i.status === status) : all
  json(res, 200, { ok: true, items: list.slice(0, 100) })
}

async function handleIssueHandle(req, res) {
  const b = await body(req)
  const id = b.id
  const list = store.read('issues.json', [])
  const item = list.find(i => i.id === id)
  if (!item) return json(res, 404, { ok: false, error: '工单不存在' })
  if (item.status === '已闭环') return json(res, 400, { ok: false, error: '该工单已闭环' })

  item.status = '已闭环'
  item.handler = '安全科'
  item.handledAt = new Date().toISOString()
  item.handleNote = String(b.note || '').slice(0, 300)
  item.handler = b.handler || '安全科'
  store.write('issues.json', list)
  log('issue.jsonl', { id, action: 'closed', note: item.handleNote })
  json(res, 200, { ok: true, item })
}

function handleIssueStats(res) {
  const all = store.read('issues.json', [])
  const closed = all.filter(i => i.status === '已闭环').length
  const byType = {}
  all.forEach(i => { byType[i.type] = (byType[i.type] || 0) + 1 })
  const byTeam = {}
  all.forEach(i => { byTeam[i.team] = (byTeam[i.team] || 0) + 1 })
  json(res, 200, {
    ok: true,
    total: all.length,
    open: all.length - closed,
    closed,
    rate: all.length ? Math.round((closed / all.length) * 100) : 0,
    byType,
    byTeam
  })
}

// ---------------- 制度宣贯 ----------------
function noticeWithStats(n, reads) {
  const base = n.teamBase || {}
  const done = reads[n.id] || {}
  const teams = Object.keys(base).map(team => {
    const total = base[team]
    const d = Math.min(total, done[team] || 0)   // 只统计真实答题人次
    return { team, total, done: d, rate: total ? Math.round((d / total) * 100) : 0 }
  })
  const totalAll = teams.reduce((a, t) => a + t.total, 0)
  const doneAll = teams.reduce((a, t) => a + t.done, 0)
  return {
    id: n.id, title: n.title, tag: n.tag, deadline: n.deadline, jobs: n.jobs,
    questions: n.questions, points: n.points, body: n.body,
    total: totalAll, done: doneAll,
    rate: totalAll ? Math.round((doneAll / totalAll) * 100) : 0,
    teams
  }
}

function handleNoticeList(res, url) {
  const job = url.searchParams.get('job') || ''
  const reads = store.read('notice-reads.json', {})
  let list = NOTICES
  if (job) list = list.filter(n => !n.jobs || n.jobs.includes(job) || n.jobs.includes('通用'))
  json(res, 200, { ok: true, items: list.map(n => noticeWithStats(n, reads)) })
}

async function handleNoticeRead(req, res) {
  const b = await body(req)
  const id = b.id
  const team = b.team || '未填区队'
  if (!NOTICES.find(n => n.id === id)) return json(res, 404, { ok: false, error: '制度不存在' })
  const reads = store.read('notice-reads.json', {})
  reads[id] = reads[id] || {}
  reads[id][team] = (reads[id][team] || 0) + 1
  store.write('notice-reads.json', reads)
  log('notice.jsonl', { id, team, score: b.score, total: b.total })
  json(res, 200, { ok: true })
}

// ---------------- 工具 ----------------
const HITS = new Map()
function rateLimited(ip) {
  const now = Date.now()
  const arr = (HITS.get(ip) || []).filter(t => now - t < 60000)
  arr.push(now)
  HITS.set(ip, arr)
  return arr.length > CFG.rateLimitPerMin
}

function log(file, obj) {
  const dir = path.join(ROOT, CFG.storageDir, 'logs')
  if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true })
  const rec = chainAppend(file, { t: new Date().toISOString(), ...obj })
  fs.appendFileSync(path.join(dir, file), JSON.stringify(rec) + '\n')
}

function json(res, code, body) {
  const s = JSON.stringify(body)
  res.writeHead(code, {
    'Content-Type': 'application/json; charset=utf-8',
    'Content-Length': Buffer.byteLength(s),
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Headers': 'Content-Type',
    'Access-Control-Allow-Methods': 'GET,POST,OPTIONS'
  })
  res.end(s)
}

function body(req, maxLen) {
  const cap = Number(maxLen) || 1e6
  return new Promise((resolve, reject) => {
    let b = ''
    req.on('data', c => {
      b += c
      if (b.length > cap) reject(new Error('请求体过大'))
    })
    req.on('end', () => {
      try {
        const o = b ? JSON.parse(b) : {}
        req.body_user = o.user || ''
        req.body_team = o.team || ''
        req.body_spent = o.spent
        resolve(o)
      } catch (e) {
        const err = new Error('请求体不是合法 JSON（检查引号、逗号和编码）')
        err.statusCode = 400
        reject(err)
      }
    })
    req.on('error', reject)
  })
}

// ---------------- 业务 ----------------
async function handleChat(req, res) {
  const ip = req.socket.remoteAddress || 'unknown'
  if (rateLimited(ip)) return json(res, 429, { ok: false, error: '问得太快了，歇一会儿再问' })

  const { question, history = [], job = '', image = '' } = await body(req)
  if (!question || String(question).trim().length < 2) {
    return json(res, 400, { ok: false, error: '问题太短了' })
  }
  // 附图：小程序先上传到 /uploads/，这里读回来转成 base64 给模型
  let imageData = ''
  const imgRef = String(image || '').trim()
  try {
    if (imgRef.startsWith('/uploads/')) {
      const fp = path.join(UPLOAD_DIR, path.basename(imgRef))
      const buf = fs.readFileSync(fp)
      if (buf.length <= 4 * 1024 * 1024) {
        const ext = path.extname(fp).slice(1).toLowerCase()
        const mime = ext === 'png' ? 'image/png' : ext === 'webp' ? 'image/webp' : 'image/jpeg'
        imageData = 'data:' + mime + ';base64,' + buf.toString('base64')
      }
    } else if (imgRef.startsWith('data:image/')) {
      imageData = imgRef
    }
  } catch (e) { /* 图片读不到就只按文字答 */ }
  const cites = retrieve(question + ' ' + job, CFG.topK)
  const started = Date.now()

  // 没配密钥时，用知识库片段拼一条"演示回答"，方便先看界面
  if (!CFG.apiKey && CFG.mock) {
    const bodyText = cites.length
      ? cites.slice(0, 2).map((c, i) => `${i + 1}. ${c.text.slice(0, 90)}…`).join('\n')
      : '知识库里没有这条内容。'
    return json(res, 200, {
      ok: true,
      mock: true,
      answer: `（演示回答 · 未接入模型）\n先看规程怎么说：\n${bodyText}\n\n配置 DeepSeek 密钥后，这里会由模型结合知识库生成正式回答。`,
      cites: cites.map(c => ({ id: c.id, title: c.title, snippet: c.text.slice(0, 120) }))
    })
  }

  if (!CFG.apiKey) {
    return json(res, 503, {
      ok: false,
      error: 'NO_KEY',
      message: '服务端还没配置 DeepSeek 密钥。把 server/config.json 里的 apiKey 填上再重启服务。'
    })
  }

  try {
    const { text, usage } = await callDeepSeek(buildMessages(question, history, cites, imageData))
    log('chat.jsonl', {
      q: question, job, cites: cites.map(c => c.id), image: !!imageData,
      a: String(text).slice(0, 600),
      ms: Date.now() - started, usage
    })
    return json(res, 200, {
      ok: true,
      grounded: cites.length > 0,
    answer: text,
      cites: cites.map(c => ({ id: c.id, title: c.title, snippet: c.text.slice(0, 120) }))
    })
  } catch (e) {
    const detail = e.message || ''
    let hint = 'AI 暂时不可用'
    if (/401/.test(detail)) {
      hint = '密钥无效（401）：DeepSeek 拒绝了这串 key，请确认它以 sk- 开头'
    } else if (/402/.test(detail)) {
      hint = '余额不足（402）：去 DeepSeek 平台充值后再试'
    } else if (/429/.test(detail)) {
      hint = '调用太频繁（429）：等一分钟再问'
    } else if (/超时|timeout/i.test(detail)) {
      hint = '调用超时：网络不稳或服务繁忙，稍后重试'
    } else if (/ENOTFOUND|EAI_AGAIN|ECONNREFUSED/.test(detail)) {
      hint = '连不上 DeepSeek：检查这台机器的外网'
    }
    log('error.jsonl', { q: question, msg: detail })
    console.error('[chat]', detail)
    return json(res, 502, { ok: false, error: hint, detail })
  }
}

async function handleQuiz(req, res) {
  const { score = 0, total = 0, wrong = [], job = '', notice = '' } = await body(req)
  const w = workerOf(req)
  const rec = {
    id: 'Q' + Date.now(),
    user: (w && w.name) || (w && w.phoneMask) || req.body_user || '未绑定',
    team: (w && w.team) || req.body_team || '未登记',
    job: job || '通用',
    notice: notice || '',
    score: Number(score) || 0,
    total: Number(total) || 0,
    spent: Number(req.body_spent) || 0,
    at: new Date().toISOString()
  }
  const list = store.read('quiz-records.json', [])
  list.unshift(rec)
  store.write('quiz-records.json', list.slice(0, 2000))
  log('quiz.jsonl', { score: rec.score, total: rec.total, wrong, job: rec.job, user: rec.user })
  json(res, 200, { ok: true, item: rec })
}

function pct(a, b) {
  return b ? Math.round((a / b) * 100) : 0
}

function examStatsData() {
  const recs = examRecords()
  const byExam = {}
  recs.forEach(r => {
    const k = r.title || r.examId || '未命名考试'
    if (!byExam[k]) byExam[k] = { title: k, attempts: 0, users: new Set(), scores: [], passed: 0 }
    byExam[k].attempts++
    byExam[k].users.add(r.device || r.user || '')
    byExam[k].scores.push(r.score)
    if (r.passed) byExam[k].passed++
  })
  const items = Object.keys(byExam).map(k => {
    const v = byExam[k]
    const avg = v.scores.length ? Math.round(v.scores.reduce((a, b) => a + b, 0) / v.scores.length) : 0
    return {
      title: v.title,
      attempts: v.attempts,
      users: v.users.size,
      avg,
      passRate: v.attempts ? Math.round((v.passed / v.attempts) * 100) : 0,
      max: v.scores.length ? Math.max(...v.scores) : 0,
      min: v.scores.length ? Math.min(...v.scores) : 0
    }
  }).sort((a, b) => b.attempts - a.attempts)
  const all = recs.map(r => r.score)
  return {
    total: recs.length,
    users: new Set(recs.map(r => r.device || r.user || '')).size,
    avg: all.length ? Math.round(all.reduce((a, b) => a + b, 0) / all.length) : 0,
    passRate: recs.length ? Math.round((recs.filter(r => r.passed).length / recs.length) * 100) : 0,
    items
  }
}

function handleExamStats(res) {
  json(res, 200, Object.assign({ ok: true }, examStatsData()))
}

function exportExamCsv(res) {
  const recs = examRecords()
  const head = ['工单号', '考试', '员工', '得分', '是否及格', '用时(秒)', '交卷时间']
  const rows = recs.map(r => [
    r.id, r.title || r.examId, r.user || '', r.score,
    r.passed ? '及格' : '不及格', r.usedSec || '', r.at
  ])
  const csv = [head, ...rows].map(r => r.map(x => '"' + String(x == null ? '' : x).replace(/"/g, '""') + '"').join(',')).join('\r\n')
  res.writeHead(200, {
    'Content-Type': 'text/csv; charset=utf-8',
    'Content-Disposition': "attachment; filename*=UTF-8''" + encodeURIComponent('考试成绩.csv')
  })
  res.end('\uFEFF' + csv)
}

function handleQuizStats(res, url, req) {
  const dev = req ? deviceOf(req) : ''
  const user = url.searchParams.get('user') || (dev ? '设备' + dev.slice(-4) : '')
  const all = store.read('quiz-records.json', [])

  let mine = null
  if (user) {
    const rows = all.filter(r => r.user === user)
    const best = rows.reduce((m, r) => Math.max(m, pct(r.score, r.total)), 0)
    const pass = rows.filter(r => pct(r.score, r.total) >= 60).length
    mine = {
      attempts: rows.length,
      best,
      last: rows[0] ? { score: rows[0].score, total: rows[0].total, at: rows[0].at } : null,
      avg: rows.length ? Math.round(rows.reduce((a, r) => a + pct(r.score, r.total), 0) / rows.length) : 0,
      passRate: pct(pass, rows.length)
    }
  }

  const byTeamMap = {}
  all.forEach(r => {
    const t = byTeamMap[r.team] = byTeamMap[r.team] || { team: r.team, attempts: 0, sum: 0, pass: 0, users: new Set() }
    const p = pct(r.score, r.total)
    t.attempts++
    t.sum += p
    if (p >= 60) t.pass++
    t.users.add(r.user)
  })
  const byTeam = Object.values(byTeamMap).map(t => ({
    team: t.team, users: t.users.size, attempts: t.attempts,
    avg: Math.round(t.sum / t.attempts), passRate: pct(t.pass, t.attempts)
  })).sort((a, b) => b.avg - a.avg)

  const byJobMap = {}
  all.forEach(r => {
    const j = byJobMap[r.job] = byJobMap[r.job] || { job: r.job, attempts: 0, sum: 0, pass: 0 }
    const p = pct(r.score, r.total)
    j.attempts++
    j.sum += p
    if (p >= 60) j.pass++
  })
  const byJob = Object.values(byJobMap).map(j => ({
    job: j.job, attempts: j.attempts,
    avg: Math.round(j.sum / j.attempts), passRate: pct(j.pass, j.attempts)
  })).sort((a, b) => b.attempts - a.attempts)

  const sumPct = all.reduce((a, r) => a + pct(r.score, r.total), 0)
  json(res, 200, {
    ok: true,
    total: all.length,
    users: new Set(all.map(r => r.user)).size,
    avg: all.length ? Math.round(sumPct / all.length) : 0,
    passRate: pct(all.filter(r => pct(r.score, r.total) >= 60).length, all.length),
    mine, byTeam, byJob,
    recent: all.slice(0, 10).map(r => ({ user: r.user, team: r.team, job: r.job, score: r.score, total: r.total, at: r.at }))
  })
}

const AUDIT_FILES = {
  'chat.jsonl': 'AI问答',
  'quiz.jsonl': '答题交卷',
  'issue.jsonl': '问题工单',
  'question.jsonl': '题库变更',
  'kb.jsonl': '知识库变更',
  'auth.jsonl': '登录与密码',
  'notice.jsonl': '制度宣贯',
  'admin.jsonl': '后台操作'
}

function readJsonl(file, limit) {
  const p = path.join(ROOT, CFG.storageDir, 'logs', file)
  if (!fs.existsSync(p)) return []
  const lines = fs.readFileSync(p, 'utf8').trim().split('\n').filter(Boolean)
  const out = []
  lines.slice(-(limit || 500)).forEach(line => {
    try { out.push(JSON.parse(line)) } catch (e) { /* ignore */ }
  })
  return out
}

function handleAudit(res, url) {
  const type = url.searchParams.get('type') || ''
  const limit = Math.min(Number(url.searchParams.get('limit') || 200), 1000)
  const dir = path.join(ROOT, CFG.storageDir, 'logs')
  const rows = []
  Object.keys(AUDIT_FILES).forEach(f => {
    if (type && f !== type) return
    const p = path.join(dir, f)
    if (!fs.existsSync(p)) return
    fs.readFileSync(p, 'utf8').trim().split('\n').filter(Boolean).slice(-400).forEach(line => {
      try {
        const o = JSON.parse(line)
        rows.push({ t: o.t || '', kind: AUDIT_FILES[f], file: f, detail: o })
      } catch (e) { /* ignore */ }
    })
  })
  rows.sort((a, b) => String(b.t).localeCompare(String(a.t)))
  json(res, 200, {
    ok: true,
    total: rows.length,
    types: Object.entries(AUDIT_FILES).map(([k, v]) => ({ file: k, label: v })),
    items: rows.slice(0, limit)
  })
}

function handleStats(res) {
  const dir = path.join(ROOT, CFG.storageDir, 'logs')
  const read = f => {
    const p = path.join(dir, f)
    if (!fs.existsSync(p)) return []
    return fs.readFileSync(p, 'utf8').trim().split('\n').filter(Boolean).map(l => {
      try { return JSON.parse(l) } catch (e) { return null }
    }).filter(Boolean)
  }
  const chat = read('chat.jsonl')
  const quiz = read('quiz.jsonl')
  const today = new Date().toISOString().slice(0, 10)
  json(res, 200, {
    ok: true,
    chatTotal: chat.length,
    chatToday: chat.filter(c => c.t.startsWith(today)).length,
    quizTotal: quiz.length,
    quizToday: quiz.filter(c => c.t.startsWith(today)).length,
    kbChunks: KB.length,
    recentChat: chat.slice(-10).reverse().map(c => ({ q: c.q, cites: c.cites, ms: c.ms })),
    topQuestions: Object.entries(chat.reduce((m, c) => {
      const k = c.q.slice(0, 20)
      m[k] = (m[k] || 0) + 1
      return m
    }, {})).sort((a, b) => b[1] - a[1]).slice(0, 5)
  })
}

// ---------------- 路由 ----------------
const server = http.createServer(async (req, res) => {
  const u = new URL(req.url, `http://${req.headers.host}`)
  try {
    if (req.method === 'OPTIONS') return json(res, 204, {})

    // 登录校验
    if (needAuth(u.pathname) && !currentSession(req)) {
      const wantHtml = (req.headers.accept || '').includes('text/html')
      if (wantHtml) {
        res.writeHead(302, { Location: '/login?next=' + encodeURIComponent(u.pathname) })
        return res.end()
      }
      return json(res, 401, { ok: false, error: '未登录', login: '/login' })
    }

    // 三员分立：按角色限制接口
    const sess = currentSession(req)
    if (sess && needAuth(u.pathname) && !roleAllowed(sess.role, u.pathname)) {
      log('auth.jsonl', { action: 'denied', user: sess.user, role: sess.role, path: u.pathname })
      return json(res, 403, { ok: false, error: `当前角色（${ROLES[sess.role] || sess.role}）无权访问该功能` })
    }

    // 登录页与登录接口
    if (u.pathname === '/login') return serveHtml(res, 'login.html')
    if (u.pathname === '/api/login' && req.method === 'POST') {
      const b = await body(req)
      const lockedMin = isLocked(req)
      if (lockedMin) {
        log('auth.jsonl', { action: 'login-locked', ip: failKey(req), user: b.user || '' })
        return json(res, 429, { ok: false, error: `尝试次数过多，请 ${lockedMin} 分钟后再试` })
      }
      if (!CFG.adminPassword) {
        return json(res, 200, { ok: true, note: '未设置密码，直接进入' })
      }
      const u = verifyUser(b.user, b.pass)
      if (u) {
        if (CFG.totpSecret) {
          if (!verifyTotp(CFG.totpSecret, b.code)) {
            markFail(req)
            log('auth.jsonl', { action: 'login-2fa-fail', ip: failKey(req), user: b.user })
            return json(res, 401, { ok: false, need2fa: true, error: '动态验证码不对' })
          }
        }
        markSuccess(req)
        const t = newSession(u)
        res.setHeader('Set-Cookie', `coal_admin=${t}; Path=/; HttpOnly; Max-Age=43200; SameSite=Lax`)
        log('auth.jsonl', { action: 'login-ok', ip: failKey(req), user: u.username, role: u.role })
        return json(res, 200, { ok: true, user: u.username, role: u.role, display: u.display })
      }
      markFail(req)
      log('auth.jsonl', { action: 'login-fail', ip: failKey(req), user: b.user || '' })
      return json(res, 401, { ok: false, error: '账号或密码不对' })
    }
    if (u.pathname === '/api/password' && req.method === 'POST') return await handleChangePassword(req, res)
    if (u.pathname === '/api/2fa/setup' && req.method === 'POST') {
      const secret = b32encode(crypto.randomBytes(20))
      return json(res, 200, { ok: true, secret, uri: totpUri(secret, CFG.adminUser) })
    }
    if (u.pathname === '/api/2fa/enable' && req.method === 'POST') {
      const b2 = await body(req)
      const secret = String(b2.secret || '').trim()
      if (!secret || !verifyTotp(secret, b2.code)) {
        return json(res, 400, { ok: false, error: '验证码不对，确认手机时间是否准确' })
      }
      persistConfig({ totpSecret: secret })
      CFG.totpSecret = secret
      log('auth.jsonl', { action: '2fa-enabled', ip: failKey(req) })
      return json(res, 200, { ok: true })
    }
    if (u.pathname === '/api/2fa/disable' && req.method === 'POST') {
      const b2 = await body(req)
      if (!CFG.totpSecret) return json(res, 400, { ok: false, error: '当前未开启' })
      if (!verifyTotp(CFG.totpSecret, b2.code)) return json(res, 400, { ok: false, error: '验证码不对' })
      persistConfig({ totpSecret: '' })
      CFG.totpSecret = ''
      log('auth.jsonl', { action: '2fa-disabled', ip: failKey(req) })
      return json(res, 200, { ok: true })
    }
    if (u.pathname === '/api/audit/verify') {
      const files = ['auth.jsonl', 'kb.jsonl', 'question.jsonl', 'issue.jsonl', 'quiz.jsonl', 'chat.jsonl']
      const results = files.map(f => verifyChain(f))
      return json(res, 200, { ok: true, allOk: results.every(r => r.ok), results })
    }
    if (u.pathname === '/api/profile' && req.method === 'GET') {
      const id = identOf(req)
      if (!id) return json(res, 200, { ok: true, name: '', avatar: '' })
      const all = store.read('profiles.json', {})
      const p = all[id.key] || {}
      return json(res, 200, { ok: true, name: p.name || '', avatar: p.avatar || '', label: id.label })
    }
    if (u.pathname === '/api/profile' && req.method === 'POST') {
      const id = identOf(req)
      if (!id) return json(res, 400, { ok: false, error: '无法识别身份' })
      const b3 = await body(req)
      const all = store.read('profiles.json', {})
      const cur = all[id.key] || {}
      if (typeof b3.name === 'string') cur.name = b3.name.trim().slice(0, 20)
      if (typeof b3.avatar === 'string') cur.avatar = b3.avatar.trim().slice(0, 300)
      cur.updatedAt = new Date().toISOString()
      all[id.key] = cur
      store.write('profiles.json', all)
      return json(res, 200, { ok: true, name: cur.name || '', avatar: cur.avatar || '' })
    }
    if (u.pathname === '/api/me') {
      const me = currentSession(req)
      if (!me) return json(res, 401, { ok: false, error: '未登录' })
      return json(res, 200, {
        ok: true, user: me.user, role: me.role,
        roleName: ROLES[me.role] || me.role, display: me.display
      })
    }
    if (u.pathname === '/api/learn/read' && req.method === 'POST') return await handleLearnRead(req, res)
    if (u.pathname === '/api/wx/login' && req.method === 'POST') {
      refreshWxConfig()
      const b2 = await body(req)
      const code = String(b2.code || '').trim()
      if (!CFG.appId || !CFG.appSecret) {
        return json(res, 200, { ok: false, error: 'NOT_CONFIGURED', message: '服务端未配置 appId/appSecret，已使用本机模式' })
      }
      if (!code) return json(res, 400, { ok: false, error: '缺少 code' })
      try {
        const d2 = await wxRequest(`/sns/jscode2session?appid=${CFG.appId}&secret=${CFG.appSecret}&js_code=${code}&grant_type=authorization_code`)
        if (!d2.openid) return json(res, 400, { ok: false, error: d2.errmsg || '换取 openid 失败' })
        const token = crypto.randomBytes(18).toString('hex')
        WX_SESSIONS.set(token, { openid: d2.openid, at: new Date().toISOString() })
        touchAppUser('wx:' + d2.openid, 'wx', '微信用户' + d2.openid.slice(-4))
        log('auth.jsonl', { action: 'wx-login', openid: d2.openid.slice(0, 6) + '***' })
        return json(res, 200, {
          ok: true, token,
          user: { label: '微信用户' + d2.openid.slice(-4), openidTail: d2.openid.slice(-4) }
        })
      } catch (e) {
        return json(res, 502, { ok: false, error: e.message })
      }
    }
    if (u.pathname === '/api/employee/summary') return handleEmployeeSummary(res, u, req)
    if (u.pathname === '/api/exams') return handleExamList(res, u, req)
    if (u.pathname === '/api/exams/all') {
      return json(res, 200, { ok: true, items: EXAMS, jobs: OPTIONS.jobs || [] })
    }
    if (u.pathname === '/api/exams/save' && req.method === 'POST') {
      const b2 = await body(req)
      const it2 = b2.item || {}
      const title = String(it2.title || '').trim()
      if (!title) return json(res, 400, { ok: false, error: '考试名称不能为空' })
      const item = {
        id: String(it2.id || ('e' + Date.now())),
        title,
        job: String(it2.job || '').trim(),
        questions: Math.min(Math.max(Number(it2.questions) || 10, 1), 50),
        minutes: Math.min(Math.max(Number(it2.minutes) || 15, 1), 180),
        pass: Math.min(Math.max(Number(it2.pass) || 60, 1), 100),
        note: String(it2.note || '').trim()
      }
      const i2 = EXAMS.findIndex(x => x.id === item.id)
      if (i2 >= 0) EXAMS[i2] = item
      else EXAMS.push(item)
      fs.writeFileSync(path.join(ROOT, CONTENT_DIR, 'exams.json'),
        JSON.stringify({ note: '考试由管理员在后台创建', items: EXAMS }, null, 2), 'utf8')
      log('admin.jsonl', { action: 'exam-save', id: item.id, title })
      return json(res, 200, { ok: true, item, total: EXAMS.length })
    }
    if (u.pathname === '/api/exams/delete' && req.method === 'POST') {
      const b2 = await body(req)
      const id2 = String(b2.id || '')
      EXAMS = EXAMS.filter(x => x.id !== id2)
      fs.writeFileSync(path.join(ROOT, CONTENT_DIR, 'exams.json'),
        JSON.stringify({ note: '考试由管理员在后台创建', items: EXAMS }, null, 2), 'utf8')
      log('admin.jsonl', { action: 'exam-delete', id: id2 })
      return json(res, 200, { ok: true, total: EXAMS.length })
    }
    if (u.pathname === '/api/exam/start' && req.method === 'POST') return await handleExamStart(req, res)
    if (u.pathname === '/api/exam/submit' && req.method === 'POST') return await handleExamSubmit(req, res)
    if (u.pathname === '/api/faq') {
      const limit = Math.min(Math.max(Number(u.searchParams.get('limit') || 6), 1), 20)
      const lines = readJsonl('chat.jsonl', 2000)
      const map = new Map()
      // 归一化：去掉语气词与标点，"瓦斯超限了怎么办" 与 "瓦斯超限怎么办" 视为同一条
      const norm = q => q.replace(/[了的地得吗呢啊呀哦嘛吧\s，。？！?！.,、~]+/g, '')
      lines.forEach(r => {
        const q = String(r.q || '').trim()
        if (q.length < 4 || q.length > 60) return
        const key = norm(q)
        if (!key) return
        const prev = map.get(key)
        if (prev) {
          prev.count++
          if (r.a && r.a.length > (prev.a || '').length) prev.a = r.a
          if ((r.cites || []).length) prev.cites = r.cites
        } else {
          map.set(key, { q, a: r.a || '', count: 1, cites: r.cites || [] })
        }
      })
      const items = [...map.values()]
        // 只保留和安全相关的高频问题：有规程依据的，或被反复问到的
        .filter(x => {
          if ((x.cites || []).length > 0) return true          // 有本矿规程依据的，优先
          if (x.count < 3) return false                        // 否则至少被问过 3 次
          return /安全|瓦斯|顶板|支护|皮带|机电|通风|撤人|规程|考试|答题|隐患|防护|煤|矿|井下|工伤/.test(x.q)
        })
        .sort((a, b) => (b.count + ((b.cites || []).length ? 3 : 0)) - (a.count + ((a.cites || []).length ? 3 : 0)))
        .slice(0, limit)
        .map(x => ({
          q: x.q,
          a: (x.a || '').slice(0, 300),
          count: x.count,
          grounded: (x.cites || []).length > 0
        }))
      return json(res, 200, { ok: true, items, total: lines.length })
    }
    if (u.pathname === '/api/options') {
      return json(res, 200, {
        ok: true,
        teams: OPTIONS.teams || [],
        jobs: OPTIONS.jobs || [],
        mediaCats: OPTIONS.mediaCats || [],
        contact: OPTIONS.contact || '',
        version: OPTIONS.version || '1.0'
      })
    }
    if (u.pathname === '/api/options/save' && req.method === 'POST') {
      const b2 = await body(req)
      const clean = v => (Array.isArray(v) ? v : String(v || '').split(/[\n,，、]/))
        .map(x => String(x).trim()).filter(Boolean).slice(0, 60)
      OPTIONS = {
        note: OPTIONS.note || '',
        teams: clean(b2.teams),
        jobs: clean(b2.jobs),
        mediaCats: clean(b2.mediaCats),
        contact: String(b2.contact || '').trim().slice(0, 30),
        version: String(b2.version || OPTIONS.version || '1.0').trim().slice(0, 10)
      }
      fs.writeFileSync(path.join(ROOT, CONTENT_DIR, 'options.json'),
        JSON.stringify(OPTIONS, null, 2), 'utf8')

      // 岗位被删掉后，原属该岗位的题目自动划到「通用」（job 置空）
      const knownJobs = new Set((OPTIONS.jobs || []).map(x => String(x).trim()))
      let movedToGeneral = 0
      QUESTIONS.forEach(q => {
        const j = String(q.job || '').trim()
        if (j && j !== '通用' && !knownJobs.has(j)) {
          q.job = ''
          movedToGeneral++
        }
      })
      if (movedToGeneral) {
        saveQuestions()
        log('admin.jsonl', { action: 'questions-to-general', count: movedToGeneral })
      }

      log('admin.jsonl', { action: 'options-save', teams: OPTIONS.teams.length, jobs: OPTIONS.jobs.length })
      return json(res, 200, {
        ok: true,
        teams: OPTIONS.teams,
        jobs: OPTIONS.jobs,
        mediaCats: OPTIONS.mediaCats || [],
        contact: OPTIONS.contact || '',
        version: OPTIONS.version || '1.0',
        movedToGeneral
      })
    }
    if (u.pathname === '/api/materials') return handleMediaList(res, u)
    if (u.pathname === '/api/materials/save' && req.method === 'POST') return await handleMediaSave(req, res)
    if (u.pathname === '/api/materials/delete' && req.method === 'POST') return await handleMediaDelete(req, res)
    if (u.pathname === '/api/appusers') {
      const quiz = store.read('quiz-records.json', [])
      const issues = store.read('issues.json', [])
      const profiles = store.read('profiles.json', {})
      const list = loadAppUsers().map(u => {
        const p = profiles[u.id] || {}
        return {
          id: u.id,
          kind: u.kind,
          kindText: u.kind === 'wx' ? '微信登录' : '本机模式',
          label: u.label,
          name: p.name || '',
          avatar: p.avatar || '',
          firstAt: u.firstAt,
          lastAt: u.lastAt,
          logins: u.logins || 1,
          quizzes: quiz.filter(r => r.user === u.label).length,
          issues: issues.filter(i => i.reporter === u.label).length
        }
      }).sort((a, b) => String(b.lastAt).localeCompare(String(a.lastAt)))
      return json(res, 200, {
        ok: true,
        total: list.length,
        wx: list.filter(x => x.kind === 'wx').length,
        dev: list.filter(x => x.kind === 'dev').length,
        items: list
      })
    }
    if (u.pathname === '/api/users/list') {
      const list = loadUsers().map(u => ({
        username: u.username, display: u.display, role: u.role,
        roleName: ROLES[u.role] || u.role, disabled: !!u.disabled,
        createdAt: u.createdAt, lastLogin: u.lastLogin
      }))
      return json(res, 200, { ok: true, items: list, roles: ROLES })
    }
    if (u.pathname === '/api/users/save' && req.method === 'POST') {
      const b2 = await body(req)
      const list = loadUsers()
      const name = String(b2.username || '').trim()
      if (!/^[A-Za-z0-9_]{3,20}$/.test(name)) {
        return json(res, 400, { ok: false, error: '账号需 3–20 位字母数字下划线' })
      }
      const role = b2.role
      if (!ROLES[role]) return json(res, 400, { ok: false, error: '角色不合法' })
      const exist = list.find(u => u.username === name)
      if (exist) {
        exist.role = role
        exist.display = b2.display || exist.display
        if (b2.password) {
          if (b2.password.length < 8) return json(res, 400, { ok: false, error: '密码至少 8 位' })
          exist.salt = crypto.randomBytes(8).toString('hex')
          exist.hash = hashPass(b2.password, exist.salt)
        }
        exist.disabled = !!b2.disabled
        saveUsers(list)
        log('auth.jsonl', { action: 'user-update', target: name, role })
        return json(res, 200, { ok: true, updated: true })
      }
      if (!b2.password || b2.password.length < 8) {
        return json(res, 400, { ok: false, error: '新用户必须设置至少 8 位密码' })
      }
      list.push(newUser(name, b2.password, role, b2.display))
      saveUsers(list)
      log('auth.jsonl', { action: 'user-create', target: name, role })
      return json(res, 200, { ok: true, created: true })
    }
    if (u.pathname === '/api/users/delete' && req.method === 'POST') {
      const b2 = await body(req)
      const list = loadUsers().filter(u => u.username !== b2.username)
      if (list.length < 3) return json(res, 400, { ok: false, error: '三员账号不能少于 3 个' })
      saveUsers(list)
      log('auth.jsonl', { action: 'user-delete', target: b2.username })
      return json(res, 200, { ok: true })
    }
    if (u.pathname === '/api/2fa/status') {
      return json(res, 200, {
        ok: true,
        enabled: !!CFG.totpSecret,
        logged: !!currentSession(req)
      })
    }
    if (u.pathname === '/api/logout' && req.method === 'POST') {
      const t = parseCookies(req).coal_admin
      if (t) SESSIONS.delete(t)
      res.setHeader('Set-Cookie', 'coal_admin=; Path=/; Max-Age=0')
      return json(res, 200, { ok: true })
    }
    if (u.pathname === '/qbank') return serveHtml(res, 'qbank.html')
    if (u.pathname === '/api/health') {
      const keyState = !CFG.apiKey ? 'missing'
        : (/^sk-[A-Za-z0-9]{20,}$/.test(CFG.apiKey) ? 'configured' : 'format-warning')
      return json(res, 200, {
        ok: true,
        key: keyState,
        model: CFG.model,
        deployMode: CFG.deployMode,
        build: BUILD_TAG,
        defaultPassword: (() => {
          try {
            return loadUsers().some(u => !u.disabled && hashPass('coal2026', u.salt) === u.hash)
          } catch (e) { return false }
        })(),
        storageDir: CFG.storageDir,
        kbChunks: KB.length
      })
    }
    if (u.pathname === '/api/chat' && req.method === 'POST') return await handleChat(req, res)
    if (u.pathname === '/api/quiz' && req.method === 'POST') return await handleQuiz(req, res)
    if (u.pathname === '/api/materials/upload' && req.method === 'POST') {
      const rawName = decodeURIComponent(u.searchParams.get('name') || 'file')
      const ext = path.extname(rawName).toLowerCase()
      if (!MEDIA_EXT[ext]) return json(res, 400, { ok: false, error: '不支持的文件类型：' + ext })
      const safeBase = path.basename(rawName, ext).replace(/[^A-Za-z0-9_-]/g, '') || 'file'
      const fname = Date.now().toString(36) + '_' + safeBase.slice(0, 24) + ext
      const dest = path.join(MEDIA_DIR, fname)
      const maxBytes = 300 * 1024 * 1024
      let size = 0
      const ws = fs.createWriteStream(dest)
      req.on('data', chunk => {
        size += chunk.length
        if (size > maxBytes) { req.destroy(); try { ws.destroy() } catch (e) {} try { fs.unlinkSync(dest) } catch (e) {} }
      })
      req.pipe(ws)
      return await new Promise(resolve => {
        ws.on('finish', () => {
          if (size > maxBytes) return resolve(json(res, 413, { ok: false, error: '文件超过 300MB' }))
          log('admin.jsonl', { action: 'media-upload', fname, size })
          resolve(json(res, 200, {
            ok: true, url: '/media/' + fname, name: rawName,
            size, type: mediaTypeOf(fname)
          }))
        })
        ws.on('error', e => resolve(json(res, 500, { ok: false, error: '写入失败：' + e.message })))
      })
    }
    if (u.pathname === '/api/upload' && req.method === 'POST') {
      const b2 = await body(req, 14 * 1024 * 1024)
      const raw = String(b2.data || '')
      const m = raw.match(/^data:(image\/(png|jpe?g|webp)|audio\/(mpeg|mp3|aac|wav|amr));base64,(.+)$/)
      if (!m) return json(res, 400, { ok: false, error: '只接受图片或音频' })
      const buf = Buffer.from(m[4], 'base64')
      const isAudio = m[1].startsWith('audio/')
      const maxSize = (isAudio ? 8 : 4) * 1024 * 1024
      if (buf.length > maxSize) return json(res, 400, { ok: false, error: (isAudio ? '音频' : '图片') + '超过 ' + (isAudio ? 8 : 4) + 'MB' })
      const sub = m[2] || m[3] || ''
      let ext = sub === 'jpeg' ? 'jpg' : sub
      const name = (isAudio ? 'a' : 'p') + Date.now().toString(36) + Math.random().toString(36).slice(2, 6) + '.' + ext
      fs.writeFileSync(path.join(UPLOAD_DIR, name), buf)
      log('admin.jsonl', { action: 'upload', name, size: buf.length })
      return json(res, 200, { ok: true, url: '/uploads/' + name, size: buf.length })
    }
    if (u.pathname.startsWith('/media/')) {
      const fname = path.basename(decodeURIComponent(u.pathname.slice(7)))
      const fp = path.join(MEDIA_DIR, fname)
      if (!fname || !fs.existsSync(fp)) return json(res, 404, { ok: false, error: 'not found' })
      const ext = path.extname(fp).toLowerCase()
      const types = {
        '.mp4': 'video/mp4', '.m4v': 'video/mp4', '.mov': 'video/quicktime', '.webm': 'video/webm',
        '.pdf': 'application/pdf', '.txt': 'text/plain; charset=utf-8',
        '.jpg': 'image/jpeg', '.jpeg': 'image/jpeg', '.png': 'image/png', '.webp': 'image/webp',
        '.zip': 'application/zip',
        '.doc': 'application/msword',
        '.docx': 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
        '.xls': 'application/vnd.ms-excel',
        '.xlsx': 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
        '.ppt': 'application/vnd.ms-powerpoint',
        '.pptx': 'application/vnd.openxmlformats-officedocument.presentationml.presentation'
      }
      const total = fs.statSync(fp).size
      const ctype = types[ext] || 'application/octet-stream'
      const range = req.headers.range
      if (range) {
        const m = /bytes=(\d*)-(\d*)/.exec(range)
        let start = m && m[1] ? parseInt(m[1], 10) : 0
        let end = m && m[2] ? parseInt(m[2], 10) : total - 1
        if (isNaN(start) || start >= total) start = 0
        if (isNaN(end) || end >= total) end = total - 1
        res.writeHead(206, {
          'Content-Type': ctype,
          'Content-Range': `bytes ${start}-${end}/${total}`,
          'Accept-Ranges': 'bytes',
          'Content-Length': end - start + 1,
          'Cache-Control': 'public, max-age=86400'
        })
        return fs.createReadStream(fp, { start, end }).pipe(res)
      }
      res.writeHead(200, {
        'Content-Type': ctype,
        'Content-Length': total,
        'Accept-Ranges': 'bytes',
        'Cache-Control': 'public, max-age=86400'
      })
      return fs.createReadStream(fp).pipe(res)
    }
    if (u.pathname.startsWith('/uploads/')) {
      const fname = path.basename(decodeURIComponent(u.pathname.slice(9)))
      const fp = path.join(UPLOAD_DIR, fname)
      if (!fname || !fs.existsSync(fp)) return json(res, 404, { ok: false, error: 'not found' })
      const ext = path.extname(fp).toLowerCase()
      const ctype = ext === '.png' ? 'image/png' : ext === '.webp' ? 'image/webp'
        : ext === '.mp3' ? 'audio/mpeg' : ext === '.aac' ? 'audio/aac'
        : ext === '.wav' ? 'audio/wav' : ext === '.amr' ? 'audio/amr' : 'image/jpeg'
      res.writeHead(200, { 'Content-Type': ctype, 'Cache-Control': 'public, max-age=86400' })
      return res.end(fs.readFileSync(fp))
    }
    if (u.pathname === '/api/issue' && req.method === 'POST') return await handleIssueCreate(req, res)
    if (u.pathname === '/api/issues') return await handleIssueList(res, u)
    if (u.pathname === '/api/issue/handle' && req.method === 'POST') return await handleIssueHandle(req, res)
    if (u.pathname === '/api/issues/stats') return handleIssueStats(res)
    if (u.pathname === '/api/quiz/stats') return handleQuizStats(res, u, req)
    if (u.pathname === '/api/exam/stats') return handleExamStats(res)
    if (u.pathname === '/api/export/exam.csv') return exportExamCsv(res)
    if (u.pathname === '/admin') return adminPage(res)
    if (u.pathname === '/api/questions/export.csv') return questionCsv(res, false)
    if (u.pathname === '/api/questions/template.csv') return questionCsv(res, true)
    if (u.pathname === '/api/questions') return handleQuestions(res, u)
    if (u.pathname === '/api/questions/admin') return handleQuestionsAdmin(res, u)
    if (u.pathname === '/api/qgroups') {
      const jobF = (u.searchParams.get('job') || '').trim()
      const by = (u.searchParams.get('by') || '').trim()
      let live = QUESTIONS.filter(q => q.on !== false)
      if (by === 'job') {
        // 按岗位练题：通用永远排第一；岗位由后台「基础配置 → 岗位」决定
        const norm = q => String(q.job || '').trim()
        const adminJobs = (OPTIONS.jobs || []).map(x => String(x).trim()).filter(x => x && x !== '通用')
        const known = new Set(adminJobs)
        const isGeneral = q => {
          const j = norm(q)
          return !j || j === '通用' || !known.has(j)
        }
        const groups = [{ name: '通用', count: live.filter(isGeneral).length }]
        adminJobs.forEach(name => {
          groups.push({ name, count: live.filter(q => norm(q) === name).length })
        })
        return json(res, 200, { ok: true, by: 'job', total: live.length, groups })
      }
      if (jobF) live = live.filter(q => {
        const j = String(q.job || '').trim()
        return j === jobF || j === '通用'
      })
      const map = {}
      live.forEach(q => {
        const k = String(q.c || '').trim() || '未分类'
        map[k] = (map[k] || 0) + 1
      })
      const groups = Object.keys(map)
        .map(name => ({ name, count: map[name] }))
        .sort((a, b) => b.count - a.count)
      return json(res, 200, { ok: true, total: live.length, groups })
    }
    if (u.pathname === '/api/questions/save' && req.method === 'POST') return await handleQuestionSave(req, res)
    if (u.pathname === '/api/questions/delete' && req.method === 'POST') return await handleQuestionDelete(req, res)
    if (u.pathname === '/api/questions/toggle' && req.method === 'POST') return await handleQuestionToggle(req, res)
    if (u.pathname === '/api/questions/import' && req.method === 'POST') return await handleQuestionImport(req, res)
    if (u.pathname === '/api/questions/assign' && req.method === 'POST') {
      const b2 = await body(req)
      const to = String(b2.to || '').trim()
      if (!to) return json(res, 400, { ok: false, error: '请填写目标岗位' })
      let n = 0
      QUESTIONS.forEach(q => {
        if (!String(q.job || '').trim()) { q.job = to; n++ }
      })
      if (n) { ensureJob(to); saveQuestions() }
      log('admin.jsonl', { action: 'questions-assign', to, count: n })
      return json(res, 200, { ok: true, moved: n, total: QUESTIONS.length })
    }
    if (u.pathname === '/api/questions/setjob' && req.method === 'POST') {
      const b2 = await body(req)
      const ids = Array.isArray(b2.ids) ? b2.ids : [b2.id]
      const job = String(b2.job || '').trim()
      let n = 0
      QUESTIONS.forEach(q => { if (ids.includes(q.id)) { q.job = job; n++ } })
      if (n) saveQuestions()
      return json(res, 200, { ok: true, changed: n })
    }
    if (u.pathname === '/api/kb/pending') return json(res, 200, { ok: true, files: listPending() })
    if (u.pathname === '/api/kb/approve' && req.method === 'POST') return await handleKbApprove(req, res)
    if (u.pathname === '/api/audit') return handleAudit(res, u)
    if (u.pathname === '/api/kb/list') return json(res, 200, { ok: true, files: kbList(), chunks: KB.length })
    if (u.pathname === '/api/kb/upload' && req.method === 'POST') return await handleKbUpload(req, res)
    if (u.pathname === '/api/kb/delete' && req.method === 'POST') return await handleKbDelete(req, res)
    if (u.pathname === '/api/export/issues.csv') return exportIssues(res)
    if (u.pathname === '/api/export/quiz.csv') return exportQuiz(res)
    if (u.pathname === '/api/culture') {
      const items = CULTURE.slice().sort((a, b) => String(b.at || '').localeCompare(String(a.at || '')))
      return json(res, 200, { ok: true, total: items.length, items })
    }
    if (u.pathname === '/api/culture/save' && req.method === 'POST') {
      const b2 = await body(req)
      const it2 = b2.item || {}
      const title = String(it2.title || '').trim()
      if (!title) return json(res, 400, { ok: false, error: '标题不能为空' })
      const rec = {
        id: String(it2.id || ('c' + Date.now())),
        tag: String(it2.tag || '').trim(),
        title,
        body: String(it2.body || '').trim(),
        at: new Date().toISOString()
      }
      const i2 = CULTURE.findIndex(x => x.id === rec.id)
      if (i2 >= 0) CULTURE[i2] = rec
      else CULTURE.unshift(rec)
      fs.writeFileSync(path.join(ROOT, CONTENT_DIR, 'culture.json'),
        JSON.stringify({ note: '制度宣贯 / 企业文化，后台可编辑', items: CULTURE }, null, 2), 'utf8')
      log('admin.jsonl', { action: 'culture-save', id: rec.id, title })
      return json(res, 200, { ok: true, item: rec, total: CULTURE.length })
    }
    if (u.pathname === '/api/culture/delete' && req.method === 'POST') {
      const b2 = await body(req)
      const id2 = String(b2.id || '')
      const before = CULTURE.length
      CULTURE = CULTURE.filter(x => x.id !== id2)
      fs.writeFileSync(path.join(ROOT, CONTENT_DIR, 'culture.json'),
        JSON.stringify({ note: '制度宣贯 / 企业文化，后台可编辑', items: CULTURE }, null, 2), 'utf8')
      log('admin.jsonl', { action: 'culture-delete', id: id2 })
      return json(res, 200, { ok: true, removed: before - CULTURE.length, total: CULTURE.length })
    }
    if (u.pathname === '/api/notices') return handleNoticeList(res, u)
    if (u.pathname === '/api/notices/save' && req.method === 'POST') {
      const b2 = await body(req)
      const it2 = b2.item || {}
      const title = String(it2.title || '').trim()
      if (!title) return json(res, 400, { ok: false, error: '标题不能为空' })
      const item = {
        id: String(it2.id || ('n' + Date.now())),
        title,
        deadline: String(it2.deadline || '').slice(0, 10),
        summary: String(it2.summary || '').trim(),
        body: String(it2.body || '').trim(),
        questions: Array.isArray(it2.questions) ? it2.questions.slice(0, 20) : [],
        at: new Date().toISOString()
      }
      const i2 = NOTICES.findIndex(x => x.id === item.id)
      if (i2 >= 0) NOTICES[i2] = Object.assign(NOTICES[i2], item)
      else NOTICES.unshift(item)
      fs.writeFileSync(path.join(ROOT, CONTENT_DIR, 'notices.json'),
        JSON.stringify({ note: '制度由安全科录入', items: NOTICES }, null, 2), 'utf8')
      log('admin.jsonl', { action: 'notice-save', id: item.id, title })
      return json(res, 200, { ok: true, item, total: NOTICES.length })
    }
    if (u.pathname === '/api/notices/delete' && req.method === 'POST') {
      const b2 = await body(req)
      const id2 = String(b2.id || '')
      const before = NOTICES.length
      NOTICES = NOTICES.filter(x => x.id !== id2)
      fs.writeFileSync(path.join(ROOT, CONTENT_DIR, 'notices.json'),
        JSON.stringify({ note: '制度由安全科录入', items: NOTICES }, null, 2), 'utf8')
      log('admin.jsonl', { action: 'notice-delete', id: id2 })
      return json(res, 200, { ok: true, removed: before - NOTICES.length, total: NOTICES.length })
    }
    if (u.pathname === '/api/notice/read' && req.method === 'POST') return await handleNoticeRead(req, res)
    if (u.pathname === '/api/content') {
      const cat = u.searchParams.get('cat') || ''
      const job = u.searchParams.get('job') || ''
      let list = ARTICLES
      if (cat) list = list.filter(a => a.cat === cat)
      if (job) list = list.filter(a => !a.jobs || a.jobs.includes(job) || a.jobs.includes('通用'))
      return json(res, 200, {
        ok: true,
        cats: ['案例', '安全月', '寄语', '光荣榜', '知识卡'],
        items: list.map(a => ({
          id: a.id, cat: a.cat, title: a.title, summary: a.summary,
          cover: a.cover, minutes: a.minutes, date: a.date
        }))
      })
    }
    if (u.pathname.startsWith('/api/content/')) {
      const id = decodeURIComponent(u.pathname.replace('/api/content/', ''))
      const item = ARTICLES.find(a => a.id === id)
      if (!item) return json(res, 404, { ok: false, error: '内容不存在' })
      return json(res, 200, { ok: true, item })
    }
    if (u.pathname === '/api/risks') {
      const job = u.searchParams.get('job') || ''
      const list = job ? RISKS.filter(r => !r.job || r.job === job) : RISKS
      return json(res, 200, { ok: true, jobs: OPTIONS.jobs || [], items: list })
    }
    if (u.pathname.startsWith('/api/risks/')) {
      const id = decodeURIComponent(u.pathname.replace('/api/risks/', ''))
      const item = RISKS.find(r => r.id === id)
      if (!item) return json(res, 404, { ok: false, error: '风险点不存在' })
      return json(res, 200, { ok: true, item })
    }
    if (u.pathname === '/api/training') {
      return json(res, 200, { ok: true, plans: TRAINING })
    }
    if (u.pathname === '/api/reload' && req.method === 'POST') {
      reloadContent()
      return json(res, 200, { ok: true, articles: ARTICLES.length, risks: RISKS.length })
    }
    if (u.pathname === '/api/stats') return handleStats(res)
    json(res, 404, { ok: false, error: 'not found' })
  } catch (e) {
    if (String(e.message || '').includes('请求体过大')) {
      try { return json(res, 413, { ok: false, error: '文件太大，请压缩后重试（图片<4MB，音频<8MB）' }) } catch (_) {}
    }
    const code = e.statusCode || 500
    if (code >= 500) console.error('[500]', e.message)
    else console.warn('[400]', e.message)
    if (!res.headersSent) json(res, code, { ok: false, error: e.message })
  }
})

// 兜底：未捕获异常不带走服务
process.on('unhandledRejection', (e) => {
  console.error('[unhandledRejection]', (e && e.message) || String(e))
})
process.on('uncaughtException', (e) => {
  console.error('[uncaughtException]', (e && e.message) || String(e))
})

server.listen(PORT, () => {
  console.log(`\n  煤矿安全员 AI 智能体 · 后端已启动`)
  console.log(`  http://127.0.0.1:${PORT}`)
  const k = CFG.apiKey
  const kMsg = !k ? '未配置（去 server/config.json 填 apiKey）'
    : (/^sk-[A-Za-z0-9]{20,}$/.test(k) ? '已配置' : '格式可疑！DeepSeek 的 key 一定以 sk- 开头，请重新复制')
  console.log(`  密钥：${kMsg}`)
  console.log(`  版本：${BUILD_TAG}`)
  console.log(`  部署模式：${CFG.deployMode === 'lan' ? '矿内局域网' : '公网云'}  模型地址：${CFG.baseUrl}`)
  console.log(`  数据目录：${path.join(ROOT, CFG.storageDir)}`)
  console.log(`  管理后台：http://127.0.0.1:${PORT}/admin`)
  console.log(`  题库：${QUESTIONS.length} 道`)
  if (CFG.adminPassword === 'coal2026') {
    console.log('  ⚠ 管理后台仍是默认密码（coal2026），请登录后到「修改密码」更换')
  }
  console.log(`  知识库：${KB.length} 个片段\n`)
})
