# 后端代理（DeepSeek 接入）

小程序不能直接持有 API Key（代码可被反编译），所以加这一层：

```
小程序  →  本服务（持密钥、查知识库、限流、记日志）  →  DeepSeek API
```

## 一、启动（双击就行，不用命令行）

**双击「启动服务.bat」**，会弹一个黑窗口，看到下面这行就算起来了：

```
地址: http://127.0.0.1:8789
```

黑窗口别关，关了服务就停。

> 你的电脑上 `node` 不在 PATH 里，bat 会自动去找 DSH 自带的 Node，不用你装。
> 如果哪天 DSH 卸载了，就装一个 LTS 版 Node（https://nodejs.org），bat 会优先用它。

浏览器打开 http://127.0.0.1:8789/api/health 可自检：
`"key":"missing"` = 还没填密钥，`"key":"configured"` = 已生效。

## 二、填 DeepSeek 密钥

**双击「填密钥.bat」** → 记事本自动打开 `config.json`，改这一行：

```json
  "apiKey": "",              ← 改之前

  "apiKey": "sk-你的密钥",    ← 改之后
```

- 引号要留着，key 前后**不能有空格**
- Ctrl+S 保存，关掉记事本，**再双击「启动服务.bat」重启**才生效
- key 在 https://platform.deepseek.com → API Keys 里生成，只显示一次

不用手改 JSON 也行：直接编辑同目录 `config.json` 是一样的效果。

进阶：

- 也可以用环境变量，优先级高于 config.json：`$env:DEEPSEEK_API_KEY="sk-..."` 再启动
- **不要**把带密钥的 config.json 发群里、发微信、提交到 git

> `mock: true` 且没填密钥时，服务会用知识库片段拼一条"演示回答"，
> 方便你先看小程序界面，不花一分钱。填了密钥后 mock 自动失效。

## 三、接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/health` | 健康检查，返回密钥是否已配、知识库片段数 |
| POST | `/api/chat` | `{"question":"...","history":[...]}` → `{ok, answer, cites}` |
| POST | `/api/quiz` | `{"score":4,"total":5,"wrong":["y1"],"job":"运输"}` |
| GET | `/api/stats` | 问答量、答题量、高频问题 TOP5 |

日志落在 `logs/`：`chat.jsonl`（每次问答）、`quiz.jsonl`（每次交卷）、`error.jsonl`。

## 四、换知识库（重要）

`kb/` 目录下每个 `.md` / `.txt`，按**空行分段**，每段自动成为一个检索块。

1. 把矿上的《煤矿安全规程》摘录、岗位操作规程、事故案例放进去
2. 每段控制在 100–300 字，一段只讲一件事，检索命中率最高
3. 改完重启服务

现在的 `kb/` 里是**示例内容**，正式交付前必须换成矿上审核过的原文。

## 五、上线前要补的

- [ ] 换成 HTTPS + 已备案域名，并在小程序后台加 `request` 白名单
- [ ] 给接口加登录态校验（现在只按 IP 限流）
- [ ] 密钥放环境变量，不进代码库
- [ ] 用 pm2 / systemd 守护进程，别用 `node index.js` 裸跑
- [ ] 问答日志接入矿上审计，按等保要求留存 6 个月
