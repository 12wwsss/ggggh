import { spawn } from 'node:child_process'
import { mkdtempSync, existsSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import http from 'node:http'

const base = 'http://127.0.0.1:8080'
const exe = ['C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe'].find((p) => existsSync(p))
const port = 9351
const child = spawn(exe, ['--headless=new', `--remote-debugging-port=${port}`, `--user-data-dir=${mkdtempSync(join(tmpdir(), 'dbg-'))}`, '--no-first-run', '--disable-gpu', 'about:blank'], { stdio: 'ignore' })
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
const getJSON = (path) => new Promise((res, rej) => http.get({ host: '127.0.0.1', port, path }, (r) => { let s = ''; r.on('data', (c) => (s += c)); r.on('end', () => res(JSON.parse(s))) }).on('error', rej))

class CDP {
  constructor(url) { this.ws = new WebSocket(url); this.id = 0; this.p = new Map(); this.ws.onmessage = (e) => { const m = JSON.parse(e.data); if (m.id && this.p.has(m.id)) { const { res, rej } = this.p.get(m.id); this.p.delete(m.id); m.error ? rej(new Error(m.error.message)) : res(m.result) } else if (m.method === 'Runtime.consoleAPICalled') { console.log('CONSOLE:', m.params.args.map(a => a.value ?? a.description).join(' ').slice(0, 200)) } else if (m.method === 'Runtime.exceptionThrown') { console.log('EXCEPTION:', m.params.exceptionDetails.exception?.description?.slice(0, 300)) } } }
  ready() { return new Promise((r) => (this.ws.onopen = r)) }
  send(method, params = {}) { const id = ++this.id; return new Promise((res, rej) => { this.p.set(id, { res, rej }); this.ws.send(JSON.stringify({ id, method, params })) }) }
  ev(expr) { return this.send('Runtime.evaluate', { expression: expr, awaitPromise: true, returnByValue: true }).then(r => r.result?.value) }
}

const token = (await (await fetch(base + '/api/v1/auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username: 'admin', password: 'Admin@123456' }) })).json()).data.token

for (let i = 0; i < 40; i++) { try { await getJSON('/json/version'); break } catch { await sleep(200) } }
const tabs = await getJSON('/json/list')
const cdp = new CDP(tabs.find(t => t.type === 'page').webSocketDebuggerUrl)
await cdp.ready()
await cdp.send('Page.enable'); await cdp.send('Runtime.enable')
await cdp.send('Emulation.setDeviceMetricsOverride', { width: 1440, height: 900, deviceScaleFactor: 1, mobile: false })
await cdp.send('Page.navigate', { url: base + '/admin/issues' })
await sleep(1500)
await cdp.ev(`localStorage.setItem('coalrag.token', ${JSON.stringify(token)}); localStorage.setItem('coalrag.user', ${JSON.stringify(JSON.stringify({ role: 'admin', name: 'x' }))}); 1`)
await cdp.send('Page.navigate', { url: base + '/admin/issues' })
await sleep(2500)
const clicked = await cdp.ev(`(() => { const row=[...document.querySelectorAll('.el-table__row')].find(r=>r.innerText.includes('已闭环')); if(!row) return 'no-row'; const b=[...row.querySelectorAll('button')].find(x=>x.textContent.trim()==='查看'); if(!b) return 'no-btn'; b.click(); return 'clicked' })()`)
console.log('click:', clicked)
await sleep(2000)
console.log('drawers:', await cdp.ev(`document.querySelectorAll('.el-drawer').length`))
console.log('overlays:', await cdp.ev(`document.querySelectorAll('.el-overlay').length`))
console.log('drawerText:', await cdp.ev(`document.querySelector('.el-drawer')?.innerText.slice(0,150) || 'none'`))
child.kill()
process.exit(0)
