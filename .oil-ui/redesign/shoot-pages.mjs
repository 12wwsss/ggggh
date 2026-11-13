// 一次拍下管理后台各页面，用于改版自查（不进入正式交付物）。
import { spawn } from 'node:child_process'
import { existsSync, mkdirSync, writeFileSync, mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import http from 'node:http'

const base = 'http://127.0.0.1:8080'
const outDir = 'D:/rag/.oil-ui/redesign/pages'
const exe = [
  'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
  'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
].find((p) => existsSync(p))
if (!exe) throw new Error('no browser')

const port = 9345
const profile = mkdtempSync(join(tmpdir(), 'coalrag-shoot-'))
const child = spawn(
  exe,
  ['--headless=new', `--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, '--no-first-run',
    '--disable-gpu', '--hide-scrollbars', 'about:blank'],
  { stdio: 'ignore' },
)
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

function getJSON(path) {
  return new Promise((resolve, reject) => {
    http.get({ host: '127.0.0.1', port, path }, (res) => {
      let s = ''
      res.on('data', (c) => (s += c))
      res.on('end', () => {
        try { resolve(JSON.parse(s)) } catch (e) { reject(e) }
      })
    }).on('error', reject)
  })
}

class CDP {
  constructor(url) {
    this.ws = new WebSocket(url)
    this.id = 0
    this.pending = new Map()
    this.ws.onmessage = (ev) => {
      const m = JSON.parse(ev.data)
      if (m.id && this.pending.has(m.id)) {
        const { resolve, reject } = this.pending.get(m.id)
        this.pending.delete(m.id)
        m.error ? reject(new Error(m.error.message)) : resolve(m.result)
      }
    }
  }
  ready() { return new Promise((res, rej) => { this.ws.onopen = res; this.ws.onerror = rej }) }
  send(method, params = {}) {
    const id = ++this.id
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject })
      this.ws.send(JSON.stringify({ id, method, params }))
    })
  }
  async evaluate(expression) {
    const r = await this.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true })
    return r.result?.value
  }
}

async function main() {
  for (let i = 0; i < 60; i++) {
    try { await getJSON('/json/version'); break } catch { await sleep(250) }
  }
  const tabs = await getJSON('/json/list')
  const cdp = new CDP(tabs.find((t) => t.type === 'page').webSocketDebuggerUrl)
  await cdp.ready()
  await cdp.send('Page.enable')
  await cdp.send('Runtime.enable')
  await cdp.send('Emulation.setDeviceMetricsOverride', { width: 1440, height: 900, deviceScaleFactor: 1, mobile: false })

  const resp = await fetch(`${base}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: 'admin', password: 'Admin@123456' }),
  })
  const { data } = await resp.json()
  mkdirSync(outDir, { recursive: true })

  await cdp.send('Page.navigate', { url: `${base}/admin/dashboard` })
  await sleep(1200)
  await cdp.evaluate(
    `localStorage.setItem('coalrag.token', ${JSON.stringify(data.token)});
     localStorage.setItem('coalrag.user', ${JSON.stringify(JSON.stringify(data.user))}); 'ok'`,
  )

  const pages = ['dashboard', 'docs', 'qa', 'learn', 'exams', 'broadcasts', 'issues', 'org', 'system']
  for (const p of pages) {
    await cdp.send('Page.navigate', { url: `${base}/admin/${p}` })
    await sleep(1800)
    const shot = await cdp.send('Page.captureScreenshot', { format: 'png' })
    writeFileSync(join(outDir, `${p}.png`), Buffer.from(shot.data, 'base64'))
    console.log('shot', p)
  }
  child.kill()
  process.exit(0)
}

main().catch((e) => { console.error(e.message); child.kill(); process.exit(1) })
