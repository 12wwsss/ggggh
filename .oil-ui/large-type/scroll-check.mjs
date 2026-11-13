import { spawn } from 'node:child_process'
import { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import http from 'node:http'

const base = 'http://127.0.0.1:8080'
const exe = 'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe'
const port = 9371
const child = spawn(exe, ['--headless=new', `--remote-debugging-port=${port}`, `--user-data-dir=${mkdtempSync(join(tmpdir(), 'chk-'))}`, '--no-first-run', '--disable-gpu', 'about:blank'], { stdio: 'ignore' })
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
const getJSON = (p) => new Promise((res, rej) => http.get({ host: '127.0.0.1', port, path: p }, (r) => { let s = ''; r.on('data', (c) => (s += c)); r.on('end', () => res(JSON.parse(s))) }).on('error', rej))

class CDP {
  constructor(u) { this.ws = new WebSocket(u); this.id = 0; this.p = new Map(); this.ws.onmessage = (e) => { const m = JSON.parse(e.data); if (m.id && this.p.has(m.id)) { const { res, rej } = this.p.get(m.id); this.p.delete(m.id); m.error ? rej(new Error(m.error.message)) : res(m.result) } } }
  ready() { return new Promise((r) => (this.ws.onopen = r)) }
  send(method, params = {}) { const id = ++this.id; return new Promise((res, rej) => { this.p.set(id, { res, rej }); this.ws.send(JSON.stringify({ id, method, params })) }) }
  ev(e) { return this.send('Runtime.evaluate', { expression: e, awaitPromise: true, returnByValue: true }).then((r) => r.result?.value) }
}

const login = await (await fetch(base + '/api/v1/auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username: 'worker01', password: 'Worker@123456' }) })).json()
for (let i = 0; i < 40; i++) { try { await getJSON('/json/version'); break } catch { await sleep(200) } }
const tabs = await getJSON('/json/list')
const cdp = new CDP(tabs.find((t) => t.type === 'page').webSocketDebuggerUrl)
await cdp.ready()
await cdp.send('Page.enable'); await cdp.send('Runtime.enable')
await cdp.send('Emulation.setDeviceMetricsOverride', { width: 390, height: 844, deviceScaleFactor: 2, mobile: true })
await cdp.send('Page.navigate', { url: base + '/' })
await sleep(1000)
await cdp.ev(`localStorage.setItem('coalrag.token', ${JSON.stringify(login.data.token)});
  localStorage.setItem('coalrag.user', ${JSON.stringify(JSON.stringify(login.data.user))});
  localStorage.setItem('coalrag.large', '1'); 'ok'`)
await cdp.send('Page.navigate', { url: base + '/' })
await sleep(2500)
const info = await cdp.ev(`(() => {
  const page = document.querySelector('.page')
  const before = { scrollH: page.scrollHeight, clientH: page.clientHeight, scrollTop: page.scrollTop }
  page.scrollTop = page.scrollHeight
  const rows = document.querySelectorAll('.lt-recent-row')
  const last = rows[rows.length - 1]
  const tab = document.querySelector('.van-tabbar')
  const r = last ? last.getBoundingClientRect() : null
  const tb = tab ? tab.getBoundingClientRect() : null
  return { ...before, afterScrollTop: page.scrollTop, lastBottom: r ? Math.round(r.bottom) : null, tabTop: tb ? Math.round(tb.top) : null }
})()`)
console.log(JSON.stringify(info))
child.kill()
process.exit(0)
