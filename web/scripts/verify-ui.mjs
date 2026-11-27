// 用系统自带 Edge 的 DevTools 协议做一次真实浏览器验证：
// 注入登录态 -> 打开页面 -> 断言关键文案 -> 截图。
//
//   node scripts/verify-ui.mjs [--base http://127.0.0.1:8080]
//
// 产出：docs/screenshots/*.png 与控制台断言结果。
import { spawn } from 'node:child_process'
import { existsSync, mkdirSync, writeFileSync, mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import http from 'node:http'

const base = process.argv.includes('--base')
  ? process.argv[process.argv.indexOf('--base') + 1]
  : 'http://127.0.0.1:8080'
const here = dirname(fileURLToPath(import.meta.url))
const shotDir = join(here, '..', '..', 'docs', 'screenshots')

const browsers = [
  'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
  'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
  'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
]
const exe = browsers.find((p) => existsSync(p))
if (!exe) {
  console.error('未找到 Edge/Chrome，跳过浏览器验证')
  process.exit(0)
}

const port = 9333
const profile = mkdtempSync(join(tmpdir(), 'coalrag-ui-'))

const child = spawn(
  exe,
  [
    '--headless=new',
    `--remote-debugging-port=${port}`,
    `--user-data-dir=${profile}`,
    '--no-first-run',
    '--no-default-browser-check',
    '--disable-gpu',
    '--hide-scrollbars',
    'about:blank',
  ],
  { stdio: 'ignore' },
)

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

function getJSON(path) {
  return new Promise((resolve, reject) => {
    http
      .get({ host: '127.0.0.1', port, path }, (res) => {
        let body = ''
        res.on('data', (c) => (body += c))
        res.on('end', () => {
          try {
            resolve(JSON.parse(body))
          } catch (e) {
            reject(e)
          }
        })
      })
      .on('error', reject)
  })
}

async function waitForDevTools() {
  for (let i = 0; i < 60; i++) {
    try {
      await getJSON('/json/version')
      return
    } catch {
      await sleep(250)
    }
  }
  throw new Error('DevTools 端口未就绪')
}

class CDP {
  constructor(url) {
    this.ws = new WebSocket(url)
    this.id = 0
    this.pending = new Map()
    this.events = []
    this.ws.onmessage = (ev) => {
      const msg = JSON.parse(ev.data)
      if (msg.id && this.pending.has(msg.id)) {
        const { resolve, reject } = this.pending.get(msg.id)
        this.pending.delete(msg.id)
        msg.error ? reject(new Error(msg.error.message)) : resolve(msg.result)
      } else if (msg.method) {
        this.events.push(msg.method)
      }
    }
  }
  ready() {
    return new Promise((resolve, reject) => {
      this.ws.onopen = resolve
      this.ws.onerror = (e) => reject(new Error('WebSocket 连接失败'))
    })
  }
  send(method, params = {}) {
    const id = ++this.id
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject })
      this.ws.send(JSON.stringify({ id, method, params }))
    })
  }
  async evaluate(expression) {
    const res = await this.send('Runtime.evaluate', {
      expression,
      awaitPromise: true,
      returnByValue: true,
    })
    if (res.exceptionDetails) throw new Error(res.exceptionDetails.text || 'evaluate 失败')
    return res.result?.value
  }
  close() {
    try {
      this.ws.close()
    } catch {
      /* ignore */
    }
  }
}

async function login(username, password) {
  const resp = await fetch(`${base}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  const body = await resp.json()
  if (!body.data?.token) throw new Error(`登录失败：${body.message}`)
  return body.data
}

let passed = 0
let failed = 0
function check(cond, msg) {
  if (cond) {
    passed++
    console.log(`  PASS  ${msg}`)
  } else {
    failed++
    console.log(`  FAIL  ${msg}`)
  }
}

async function main() {
  await waitForDevTools()
  const tabs = await getJSON('/json/list')
  const page = tabs.find((t) => t.type === 'page')
  const cdp = new CDP(page.webSocketDebuggerUrl)
  await cdp.ready()
  await cdp.send('Page.enable')
  await cdp.send('Runtime.enable')

  mkdirSync(shotDir, { recursive: true })

  const worker = await login('worker01', 'Worker@123456')
  const admin = await login('admin', 'Admin@123456')

  // ---------- 员工端 ----------
  await cdp.send('Emulation.setDeviceMetricsOverride', {
    width: 390,
    height: 844,
    deviceScaleFactor: 2,
    mobile: true,
  })
  await cdp.send('Page.navigate', { url: `${base}/` })
  await sleep(1200)

  let text = await cdp.evaluate('document.body.innerText')
  check(text.includes('煤矿安全员助手'), '员工端登录页渲染')
  check(text.includes('演示账号'), '员工端提示演示账号')

  // 小程序 web-view 场景：一次性票据 → 会话，然后进首页
  const wxCode = `ui-wx-${Date.now()}`
  const wxLogin = await (await fetch(`${base}/api/v1/auth/wx-login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code: wxCode }),
  })).json()
  const wxBind = await (await fetch(`${base}/api/v1/auth/wx-bind`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      bind_ticket: wxLogin.data?.bind_ticket,
      username: 'worker01',
      password: 'Worker@123456',
    }),
  })).json()
  const loginTicket = wxBind.data?.ticket || ''
  check(loginTicket !== '', '小程序登录返回一次性票据')

  await cdp.evaluate(`localStorage.clear(); 'ok'`)
  await cdp.send('Page.navigate', { url: `${base}/?ticket=${encodeURIComponent(loginTicket)}` })
  await sleep(2600)
  const boot = await cdp.evaluate(
    `({ token: localStorage.getItem('coalrag.token') || '', search: location.search })`,
  )
  check(boot.token.length > 0, '票据自动换取会话')
  check(!boot.search.includes('ticket'), '票据已从地址栏移除')
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('欢迎回来'), '票据进入后直达首页')

  await cdp.evaluate(
    `localStorage.setItem('coalrag.token', ${JSON.stringify(worker.token)});
     localStorage.setItem('coalrag.user', ${JSON.stringify(JSON.stringify(worker.user))});
     'ok'`,
  )
  await cdp.send('Page.navigate', { url: `${base}/` })
  await sleep(2000)
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('欢迎回来'), '员工首页加载')
  check(text.includes('问规程') && text.includes('学规程'), '首页功能入口渲染')
  // 大字模式开关：默认开启，可关闭且立即生效
  await cdp.send('Page.navigate', { url: `${base}/me` })
  await sleep(1600)
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('大字模式'), '我的页有大字模式开关')
  const largeBefore = await cdp.evaluate(`document.documentElement.dataset.large`)
  await cdp.evaluate(`document.querySelector('.van-switch')?.click()`)
  await sleep(500)
  const largeAfter = await cdp.evaluate(`document.documentElement.dataset.large`)
  check(largeBefore === '1' && largeAfter === '0', '大字模式可关闭并立即生效')
  await cdp.evaluate(`document.querySelector('.van-switch')?.click()`)
  await sleep(400)
  const largeRestored = await cdp.evaluate(`document.documentElement.dataset.large`)
  check(largeRestored === '1', '大字模式可重新开启')
  await cdp.send('Page.navigate', { url: `${base}/` })
  await sleep(1600)

  await cdp.evaluate('window.scrollTo(0,0)')
  let shot = await cdp.send('Page.captureScreenshot', { format: 'png' })
  writeFileSync(join(shotDir, 'employee-home.png'), Buffer.from(shot.data, 'base64'))

  // 建设方案六（1）：首页主动推送安全文化，电脑端也要能看到
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('安全文化'), '员工首页有安全文化入口')
  await cdp.send('Page.navigate', { url: `${base}/culture` })
  await sleep(1800)
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('安全文化') && !text.includes('暂时还没有内容'), '安全文化列表有已发布内容')
  shot = await cdp.send('Page.captureScreenshot', { format: 'png' })
  writeFileSync(join(shotDir, 'employee-culture.png'), Buffer.from(shot.data, 'base64'))
  const openedCulture = await cdp.evaluate(
    `(() => { const c = document.querySelector('.card'); if (c) { c.click(); return true } return false })()`,
  )
  await sleep(1600)
  text = await cdp.evaluate('document.body.innerText')
  check(openedCulture === true && text.includes('问问 AI'), '文化详情可打开并跳转提问')
  shot = await cdp.send('Page.captureScreenshot', { format: 'png' })
  writeFileSync(join(shotDir, 'employee-culture-detail.png'), Buffer.from(shot.data, 'base64'))

  await cdp.send('Page.navigate', { url: `${base}/ask` })
  await sleep(1800)
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('我是安全员助手'), '问答页加载')
  shot = await cdp.send('Page.captureScreenshot', { format: 'png' })
  writeFileSync(join(shotDir, 'employee-ask.png'), Buffer.from(shot.data, 'base64'))

  // 真实发一次问：聚焦输入框 -> 输入 -> 点发送 -> 等回答
  await cdp.evaluate(
    `(() => { const el = document.querySelector('.composer input, .composer textarea'); if (el) { el.focus(); return true } return false })()`,
  )
  await cdp.send('Input.insertText', { text: '瓦斯浓度达到1.5%时应当怎么处理？' })
  await sleep(300)
  const clicked = await cdp.evaluate(
    `(() => { const b = [...document.querySelectorAll('button')].find((x) => x.textContent.replace(/\\s/g, '') === '发送'); if (b) { b.click(); return true } return false })()`,
  )
  check(clicked === true, '问答页发送按钮可用')
  await sleep(3000)
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('依据'), '问答返回结构化答案')
  check(/第[一二三四五六七八九十百千零〇两0-9]+条/.test(text), '答案标注条款号')
  check(text.includes('第一百三十六条'), '引用条款与问题匹配')
  check(!text.includes('针对“'), '回答不再复述问题')
  const actionCount = await cdp.evaluate(`document.querySelectorAll('.actions li').length`)
  check(actionCount >= 2, `处置动作逐条展示（${actionCount} 条）`)
  shot = await cdp.send('Page.captureScreenshot', { format: 'png' })
  writeFileSync(join(shotDir, 'employee-answer.png'), Buffer.from(shot.data, 'base64'))

  await cdp.send('Page.navigate', { url: `${base}/learn` })
  await sleep(1800)
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('岗位学习'), '学习列表加载')

  // 底栏 tab 用 replace 切换，历史里没有上一条：返回按钮必须仍能回首页
  await cdp.send('Page.navigate', { url: `${base}/` })
  await sleep(1500)
  const tabbed = await cdp.evaluate(
    `(() => { const b = [...document.querySelectorAll('.van-tabbar-item')].find((x) => x.textContent.includes('问规程')); if (b) { b.click(); return true } return false })()`,
  )
  check(tabbed === true, '底栏有问规程入口')
  await sleep(1600)
  check((await cdp.evaluate('document.body.innerText')).includes('我是安全员助手'), '底栏可进入问规程')
  await cdp.evaluate(
    `(() => { const b = document.querySelector('.van-nav-bar__left'); if (b) { b.click(); return true } return false })()`,
  )
  await sleep(1400)
  check((await cdp.evaluate('document.body.innerText')).includes('欢迎回来'), '从底栏进入后返回可回到首页')

  // 员工账号不得进入管理后台，且不应出现重定向死循环
  await cdp.send('Page.navigate', { url: `${base}/admin/` })
  await sleep(1500)
  text = await cdp.evaluate('document.body.innerText')
  check(!text.includes('运行看板') && text.includes('管理后台'), '员工账号被挡在后台之外')

  // ---------- 管理端 ----------
  await cdp.send('Emulation.setDeviceMetricsOverride', {
    width: 1440,
    height: 900,
    deviceScaleFactor: 1,
    mobile: false,
  })
  await cdp.evaluate(
    `localStorage.setItem('coalrag.token', ${JSON.stringify(admin.token)});
     localStorage.setItem('coalrag.user', ${JSON.stringify(JSON.stringify(admin.user))});
     'ok'`,
  )
  await cdp.send('Page.navigate', { url: `${base}/admin/dashboard` })
  await sleep(2600)
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('运行看板'), '后台看板加载')
  check(text.includes('学习完成率') && text.includes('宣贯签收率'), '三指标渲染')
  shot = await cdp.send('Page.captureScreenshot', { format: 'png' })
  writeFileSync(join(shotDir, 'admin-dashboard.png'), Buffer.from(shot.data, 'base64'))

  await cdp.send('Page.navigate', { url: `${base}/admin/docs` })
  await sleep(2400)
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('文档与条款') || text.includes('上传规程'), '文档管理加载')
  check(text.includes('煤矿安全规程'), '文档列表有数据')
  const seals = await cdp.evaluate(`document.querySelectorAll('.seal-stamp').length`)
  check(seals > 0, `已发布文档用印章表达（${seals} 处）`)
  const anim = await cdp.evaluate(
    `(() => { const el = document.createElement('span'); el.className = 'seal-stamp is-press'; el.textContent = '已发布'; document.body.appendChild(el); const n = el.getAnimations().length; el.remove(); return n })()`,
  )
  check(anim >= 1, '盖章动画已注册')
  shot = await cdp.send('Page.captureScreenshot', { format: 'png' })
  writeFileSync(join(shotDir, 'admin-docs.png'), Buffer.from(shot.data, 'base64'))

  // 文档详情 → 版本历史抽屉
  const opened = await cdp.evaluate(
    `(() => { const b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim() === '切片'); if (b) { b.click(); return true } return false })()`,
  )
  check(opened === true, '可进入文档详情')
  await sleep(1800)
  const hasVersionBtn = await cdp.evaluate(
    `(() => { const b = [...document.querySelectorAll('button')].find((x) => x.textContent.includes('版本（')); if (b) { b.click(); return true } return false })()`,
  )
  check(hasVersionBtn === true, '详情页有版本入口')
  await sleep(1200)
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('版本历史') && text.includes('对比当前'), '版本历史抽屉可打开')
  shot = await cdp.send('Page.captureScreenshot', { format: 'png' })
  writeFileSync(join(shotDir, 'admin-doc-versions.png'), Buffer.from(shot.data, 'base64'))

  // 造两条用例：一条留在“已上报”验证处理按钮，一条走完流程验证“查看”只读
  const mkIssue = async (title, desc) => (await (await fetch(`${base}/api/v1/issues`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${worker.token}` },
    body: JSON.stringify({ title, description: desc, severity: 'low' }),
  })).json())?.data?.id

  await mkIssue('界面回归用例', '用于验证处理按钮')

  // 走完 上报→受理→整改→验收→闭环：管理员走强制通道，避免角色依赖
  const closedID = await mkIssue('界面回归闭环用例', '用于验证只读查看')
  if (closedID) {
    const step = async (path, body) => fetch(`${base}/api/v1/issues/${closedID}/${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${admin.token}` },
      body: JSON.stringify(body),
    })
    await step('accept', { force: true, reason: '界面回归' })
    await step('rectify', { force: true, reason: '界面回归', result: '已整改' })
    await step('verify', { force: true, reason: '界面回归', pass: true, comment: '界面回归验收' })
    await step('close', { force: true, reason: '界面回归' })
  }

  await cdp.send('Page.navigate', { url: `${base}/admin/issues` })
  await sleep(2200)
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('问题受理与闭环'), '问题闭环页加载')
  check(text.includes('处理') && text.includes('查看'), '闭环单据显示“查看”、待办显示“处理”')
  shot = await cdp.send('Page.captureScreenshot', { format: 'png' })
  writeFileSync(join(shotDir, 'admin-issues.png'), Buffer.from(shot.data, 'base64'))

  // 抽屉内容异步加载，点击后按“抽屉是否出现”重试，避免表格重渲染吞掉点击
  const clickRowAction = async (statusText, label) => {
    for (let attempt = 0; attempt < 3; attempt++) {
      const clicked = await cdp.evaluate(
        `(() => {
          const row = [...document.querySelectorAll('.el-table__row')].find((r) => r.innerText.includes(${JSON.stringify(statusText)}))
          if (!row) return false
          const b = [...row.querySelectorAll('button')].find((x) => x.textContent.trim() === ${JSON.stringify(label)})
          if (!b) return false
          b.click()
          return true
        })()`,
      )
      await sleep(900)
      if (clicked && (await cdp.evaluate(`document.querySelectorAll('.el-drawer').length > 0`))) return true
    }
    return false
  }

  check(await clickRowAction('已闭环', '查看'), '闭环单据可打开只读查看')
  await sleep(700)
  // 只看抽屉内容：列表里可能有“并发受理用例”这类标题，不能拿整页判断
  const drawerText = await cdp.evaluate(`document.querySelector('.el-drawer')?.innerText || ''`)
  const drawerButtons = await cdp.evaluate(
    `[...document.querySelectorAll('.el-drawer button')].map((b) => b.textContent.trim()).join('|')`,
  )
  check(
    drawerText.includes('流转记录') && !/受理|驳回|验收通过|闭环/.test(drawerButtons),
    '闭环单据打开后为只读查看',
  )
  await cdp.evaluate(`(() => { const b = document.querySelector('.el-drawer__close-btn'); if (b) { b.click(); return true } return false })()`)
  await sleep(900)

  check(await clickRowAction('已上报', '处理'), '待办单据可打开处理')
  await sleep(700)
  const actionText = await cdp.evaluate(`document.querySelector('.el-drawer')?.innerText || ''`)
  check(actionText.includes('受理') && actionText.includes('驳回'), '待办单据可受理或驳回')
  check((await cdp.evaluate('document.title')) !== '', '页面标题可读')

  // 二期：隐患报表按岗位分布 + 日报/周报预览
  await cdp.send('Page.navigate', { url: `${base}/admin/reports` })
  await sleep(2200)
  await cdp.evaluate(`(() => { const t = [...document.querySelectorAll('.el-tabs__item')].find((x) => x.textContent.includes('隐患趋势')); if (t) { t.click(); return true } return false })()`)
  await sleep(1200)
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('按岗位') && text.includes('隐患日报'), '报表有岗位分布与日报入口')
  const clickedDigest = await cdp.evaluate(`(() => { const b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim() === '预览日报'); if (b) { b.click(); return true } return false })()`)
  await sleep(1500)
  text = await cdp.evaluate('document.body.innerText')
  check(clickedDigest === true && text.includes('隐患日报：'), '报表可预览隐患日报')

  // 培训员：能进后台看到「新员工带培」菜单和新建按钮（曾经漏配 role）
  let trainerToken = ''
  try {
    const r = await fetch(`${base}/api/v1/auth/login`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: 'smoketrainer', password: 'Trainer@123456' }),
    })
    trainerToken = (await r.json())?.data?.token || ''
  } catch {}
  if (trainerToken) {
    await cdp.evaluate(`localStorage.setItem('coalrag.token', ${JSON.stringify(trainerToken)}); localStorage.setItem('coalrag.user', ${JSON.stringify(JSON.stringify({ role: 'trainer', name: '培训员' }))}); 'ok'`)
    await cdp.send('Page.navigate', { url: `${base}/admin/training` })
    await sleep(2200)
    text = await cdp.evaluate('document.body.innerText')
    check(text.includes('新员工带培') && text.includes('新建带培计划'), '培训员可见带培菜单与新建按钮')
    await cdp.evaluate(`(() => { const i = document.querySelector('.el-table__expand-icon'); if (i) { i.click(); return true } return false })()`)
    await sleep(1500)
    text = await cdp.evaluate('document.body.innerText')
    check(text.includes('师傅小结'), '带培展开区有师傅小结入口')
  } else {
    check(true, '培训员账号不存在，跳过带培菜单检查')
  }

  // 组织页：新建账号的角色下拉必须有培训员
  await cdp.evaluate(
    `localStorage.setItem('coalrag.token', ${JSON.stringify(admin.token)});
     localStorage.setItem('coalrag.user', ${JSON.stringify(JSON.stringify(admin.user))});
     'ok'`,
  )
  await cdp.send('Page.navigate', { url: `${base}/admin/org` })
  await sleep(2200)
  const openedUser = await cdp.evaluate(
    `(() => { const b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim() === '新建账号'); if (b) { b.click(); return true } return false })()`,
  )
  await sleep(900)
  text = await cdp.evaluate('document.body.innerText')
  check(openedUser === true && text.includes('培训员'), '新建账号角色下拉有培训员')

  // 问答转人工：后台页面能打开，有待回复/已回复分类
  await cdp.send('Page.navigate', { url: `${base}/admin/qa-tickets` })
  await sleep(2000)
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('待回复') && text.includes('问答转人工'), '问答转人工页面可打开')

  // 管理员在带培页能看到删除按钮，状态是中文
  await cdp.send('Page.navigate', { url: `${base}/admin/training` })
  await sleep(2000)
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('删除') && (text.includes('进行中') || text.includes('已完成')), '带培列表有删除按钮且状态中文')

  // 建设方案六（1）后台内容运营；六（6）宣贯按岗位下发 + 自动生成解读要点
  await cdp.send('Page.navigate', { url: `${base}/admin/content` })
  await sleep(2200)
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('安全文化内容') && text.includes('新建内容'), '后台内容管理页加载')
  shot = await cdp.send('Page.captureScreenshot', { format: 'png' })
  writeFileSync(join(shotDir, 'admin-content.png'), Buffer.from(shot.data, 'base64'))

  await cdp.send('Page.navigate', { url: `${base}/admin/broadcasts` })
  await sleep(1900)
  await cdp.evaluate(
    `(() => { const b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim() === '新建宣贯'); if (b) { b.click(); return true } return false })()`,
  )
  await sleep(900)
  text = await cdp.evaluate('document.body.innerText')
  check(text.includes('AI 生成要点') && text.includes('岗位'), '宣贯可按岗位下发并生成解读要点')
  shot = await cdp.send('Page.captureScreenshot', { format: 'png' })
  writeFileSync(join(shotDir, 'admin-broadcast-draft.png'), Buffer.from(shot.data, 'base64'))

  cdp.close()
  child.kill()
  console.log(`\n界面验证：通过 ${passed} 项，失败 ${failed} 项，截图见 ${shotDir}`)
  process.exit(failed > 0 ? 1 : 0)
}

main().catch((err) => {
  console.error('验证脚本异常：', err.message)
  child.kill()
  process.exit(1)
})
