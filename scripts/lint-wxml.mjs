// 小程序 WXML 静态检查：只查会导致「编译不过」的常见写法。
// 1) 字符串字面量下标：{{'ABCDEF'[i]}} —— WXML 表达式不支持，会报 unexpected token '['
// 2) 未闭合的 {{ }} —— 少了右括号
import { readFileSync } from 'node:fs'
import { globSync } from 'node:fs'
import { join } from 'node:path'

const root = process.argv[2] || 'miniprogram-work'
const files = globSync(`${root}/**/*.wxml`, { cwd: process.cwd() })
let bad = 0

for (const rel of files) {
  const text = readFileSync(join(process.cwd(), rel), 'utf8')
  const lines = text.split(/\r?\n/)
  lines.forEach((line, i) => {
    const literalIndex = /'[^']*'\s*\[/.exec(line)
    if (literalIndex) {
      bad++
      console.error(`${rel}:${i + 1} 字符串下标不被 WXML 支持: ${line.trim()}`)
    }
    const opens = (line.match(/\{\{/g) || []).length
    const closes = (line.match(/\}\}/g) || []).length
    if (opens !== closes) {
      bad++
      console.error(`${rel}:${i + 1} {{ }} 不配对: ${line.trim()}`)
    }
  })
}

if (bad) {
  console.error(`\nWXML 检查失败：${bad} 处`)
  process.exit(1)
}
console.log(`WXML 检查通过：${files.length} 个文件`)
