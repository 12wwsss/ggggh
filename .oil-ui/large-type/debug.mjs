import { spawn } from 'node:child_process'
import { mkdtempSync, existsSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import http from 'node:http'

const base = 'http://127.0.0.1:8080'
const exe = 'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe'
const port = 9362
const child = spawn(exe, ['--headless=new', `--remote-debugging-port=${port}`, `--user-data-dir=${mkdtempSync(join(tmpdir(), 'dbg2-'))}`, '--no-first-run', '--disable-gpu', 'about:blank'], { stdio: 'ignore' })
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
const getJSON = (path) => new Promise((res, rej) => http.get({ host: '127.0.0.1', port, path }, (r) => { let s = ''; r.on('data', (c) => (s += c)); r.on('end', () => res(JSON.parse(s))) }).on('error', rej))

class CDP {
  constructor(url) { this.ws = new WebSocket(url); this.id = 0; this.p = new Map(); this.ws.onmessage = (e) => { const m = JSON.parse(e.data); if (m.id && this.p.has(m.id)) { const { res, rej } = this.p.get(m.id); this.p.delete(m.id); m.error ? rej(new Error(m.error.message)) : res(m.result) } else if (m.method === 'Runtime.exceptionThrown') { console.log('EXC:', (m.params.exceptionDetails.exception?.description || m.params.exceptionDetails.text || '').slice(0, 400)) } else if (m.method === 'Runtime.consoleAPICalled' && m.params.type === 'error') { console.log('CONSOLE:', m.params.args.map((a) => a.value ?? a.description).join(' ').slice(0, 300)) } } }
  ready() { return new Promise((r) => (this.ws.onopen = r)) }
  send(method, params = {}) { const id = ++this.id; return new Promise((res, rej) => { this.p.set(id, { res, rej }); this.ws.send(JSON.stringify({ id, method, params })) }) }
  ev(expr) { return this.send('Runtime.evaluate', { expression: expr, awaitPromise: true, returnByValue: true }).then((r) => r.result?.value) }
}

const login = await (await fetch(base + '/api/v1/auth/login', {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ username: 'worker01', password: 'Worker@123456' }),
})).json()

for (let i = 0; i < 40; i++) { try { await getJSON('/json/version'); break } catch { await sleep(200) } }
const tabs = await getJSON('/json/list')
const cdp = new CDP(tabs.find((t) => t.type === 'page').webSocketDebuggerUrl)
await cdp.ready()
await cdp.send('Page.enable'); await cdp.send('Runtime.enable')
await cdp.send('Page.navigate', { url: base + '/' })
await sleep(1000)
await cdp.ev(`localStorage.setItem('coalrag.token', ${JSON.stringify(login.data.token)});
  localStorage.setItem('coalrag.user', ${JSON.stringify(JSON.stringify(login.data.user))});
  localStorage.setItem('coalrag.large', '1'); 'ok'`)
await cdp.send('Page.navigate', { url: base + '/' })
await sleep(3000)
console.log('appHtmlLen:', await cdp.ev(`document.querySelector('#app')?.innerHTML.length || 0`))
console.log('bodyText:', (await cdp.ev(`document.body.innerText`))?.slice(0, 200))
child.kill()
process.exit(0)
