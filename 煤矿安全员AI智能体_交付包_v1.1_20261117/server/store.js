/**
 * 轻量 JSON 存储：题库之外的业务数据（问题工单、宣贯完成记录）
 * 数据量小，直接读写文件；并发量大的场景再换数据库。
 */
const fs = require('fs')
const path = require('path')

function makeStore(dataDir) {
  const dir = path.resolve(dataDir)

  function ensure() {
    if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true })
  }

  function read(name, fallback) {
    try {
      const p = path.join(dir, name)
      if (!fs.existsSync(p)) return fallback
      return JSON.parse(fs.readFileSync(p, 'utf8'))
    } catch (e) {
      console.error('[store] 读取失败', name, e.message)
      return fallback
    }
  }

  function write(name, obj) {
    ensure()
    const p = path.join(dir, name)
    const tmp = p + '.tmp'
    fs.writeFileSync(tmp, JSON.stringify(obj, null, 2))
    fs.renameSync(tmp, p)
    return obj
  }

  return { read, write, dir }
}

module.exports = { makeStore }
