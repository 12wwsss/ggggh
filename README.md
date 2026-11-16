# 煤矿安全员AI智能体平台（后端优先版）

按《煤矿安全员AI智能体平台-技术方案》实现的服务端骨架与业务链路：

- **已实现（S0–S7 + 前端）**：环境探针、服务骨架与迁移、登录/JWT/RBAC/数据范围、文档上传与条款切分、发布审核、向量检索、RAG 问答与引用校验、岗位学习与小测、宣贯签收、在线考试、三指标看板、二期问题闭环（上报→受理→整改→验收→闭环+超时扫描）；员工 H5（Vant）与管理后台（Element Plus）两个前端，构建产物内嵌进同一个二进制。
- **本轮不做**（按需求“大模型离线部署先不弄”）：S8 目标模型接入与显存调优、S9 信创迁移、S10 隧道与云端反代。

## 本地已就绪的环境

| 组件 | 位置 | 版本 |
| --- | --- | --- |
| Go 工具链 | `.tools\go` | 1.23.5 |
| PostgreSQL（便携版） | `.tools\pgsql` | 18.6，监听 `127.0.0.1:55432` |
| 数据目录 | `.data\pg` | 开发库 `coalrag` |
| 上传与评测产物 | `storage\` | 运行时生成 |

本机开发用的便携工具链（`.tools`）与数据库数据（`.data`）都在 `.gitignore` 里，**仓库不包含**；换机器需要自备 Go 1.23+、Node 20+、PostgreSQL 18，或按下面“从源码构建”操作。


> 真实模型实测（Qwen2.5-14B Q5_K_M + bge-m3 Q8_0，双 2080 Ti）：检索 20/20、问答 30/30、平均 0.86 秒/题，详见 docs/OFFLINE-EVAL.md。

> 并发实测：问答约 3.8 QPS（8 并发内 p95 < 2.2 s）、检索约 540 QPS，详见 docs/CONCURRENCY.md。
## 从源码构建（新机器）

仓库不含 `web/dist`（前端构建产物）和便携工具链，第一次克隆后请按顺序来：

```powershell
# 1) 前端：安装依赖并构建（产物供 go:embed 打包）
cd web
pnpm install
pnpm build
cd ..

# 2) 后端：构建（dist 缺失时 go:embed 会直接报错，这是有意的）
go build -o bin\coalrag-server.exe .\cmd\server

# 3) 数据库：准备一个 PostgreSQL，设置连接串后启动
$env:COALRAG_DSN = "postgres://postgres:<口令>@127.0.0.1:5432/coalrag?sslmode=disable"
.\bin\coalrag-server.exe
```

空库首次启动会自动建表并写入演示数据（`COALRAG_BOOTSTRAP=true`）。

> 只改后端时不需要重复构建前端；改过 `web/src` 后必须重新 `pnpm build` 并重新编译二进制，
> 否则页面还是旧版本。`scripts\build-web.ps1` 封装了这一步。

## 快速开始

```powershell
# 1) 启动数据库（如未启动）+ 编译 + 启动服务
powershell -ExecutionPolicy Bypass -File scripts\dev.ps1

# 2) 浏览器打开：员工端 http://127.0.0.1:8080/ ，管理端 http://127.0.0.1:8080/admin/
#    前端已构建并内嵌在二进制里；改前端后执行 scripts\build-web.ps1 重新打包

# 3) 另一个终端跑端到端验收（103 项断言）
powershell -ExecutionPolicy Bypass -File scripts\selftest.ps1

# 4) 浏览器界面回归（Edge/Chrome 无头，39 项断言 + 截图）
node web\scripts\verify-ui.mjs --base http://127.0.0.1:8080

# 5) 单元测试
$env:PATH = "D:\rag\.tools\go\bin;$env:PATH"; go test ./...
```

空库首次启动会自动建表并写入演示数据（`COALRAG_BOOTSTRAP=true`）：

| 账号 | 口令 | 角色 | 部门 |
| --- | --- | --- | --- |
| `admin` | `Admin@123456` | 平台管理员 | 矿部 |
| `leader01` / `leader02` | `Leader@123456` | 区队长 | 综采一队 / 综采二队 |
| `worker01`–`worker03` | `Worker@123456` | 员工 | 综采一队 / 综采二队 |
| `worker04` | `Worker@123456` | 员工 | 通风区 |

演示口令仅用于开发，上线前必须改掉（`POST /api/v1/users/:id/password`）。

## 目录结构

```
cmd/server        服务入口（HTTP + 进程内任务 worker）
cmd/smoke         端到端自测（103 项断言，对应验收判据）
cmd/tune          检索阈值标定工具（换嵌入模型后重新标定 min_score）
cmd/envprobe      环境探针（S0，输出 env-report.txt）
cmd/userctl       账号急救：查看锁定状态、解锁、重置口令（直连数据库）
internal/config   配置加载：configs/config.json + 环境变量覆盖
internal/logx     slog 结构化日志与 trace id
internal/db       连接池与迁移执行器（迁移 SQL 内嵌）
internal/store    数据访问层，全部 SQL 只出现在本包
internal/auth     JWT、bcrypt、RBAC 判定表
internal/parser   Parser 接口：docx 直解 / 纯文本 / LibreOffice 子进程；条款切分
internal/llm      LLM 接口：OpenAI 兼容客户端 + 确定性桩
internal/vector   VectorStore 接口：内存实现（默认）+ pgvector 实现
internal/service  业务编排：入库、检索、问答、评测、任务 worker
internal/api      gin 路由、中间件（鉴权/限流/审计）、各域 handler
internal/bootstrap 空库初始化与向量维度校验
migrations → internal/db/migrations
web               前端：Vite + Vue3，两个入口（员工 H5 / 管理后台），dist 经 go:embed 打包
web/src/shared    HTTP 封装、会话、格式化、主题变量（两个前端共用）
web/src/employee  Vant 员工端页面
web/src/admin     Element Plus 管理后台页面
web/scripts       verify-ui.mjs：无头浏览器回归
scripts           dev / selftest / stop / build-web
docs              API 说明、实现对照、前端说明、验收输出、截图
```

## 配置

优先级：环境变量 > `configs/config.json` > 内置默认值。常用项：

| 环境变量 | 含义 | 默认 |
| --- | --- | --- |
| `COALRAG_HTTP_ADDR` | 监听地址 | `127.0.0.1:8080` |
| `COALRAG_DSN` | 数据库连接串 | 本机 55432 的 `coalrag` |
| `COALRAG_JWT_SECRET` | JWT 密钥 | 开发默认值，生产必换 |
| `COALRAG_LLM_PROVIDER` | `stub` 或 `openai` | `stub` |
| `COALRAG_LLM_BASE_URL` / `_MODEL` / `_API_KEY` | 对话模型 | 桩 |
| `COALRAG_EMBED_PROVIDER` / `_MODEL` / `_DIM` | 嵌入模型与维度 | `stub` / bge-m3 / 1024 |
| `COALRAG_VECTOR_KIND` | `memory` 或 `pgvector` | `memory` |
| `COALRAG_TOP_K` / `COALRAG_MIN_SCORE` | 检索条数与阈值 | 6 / 0.08 |
| `COALRAG_TUNNEL_ADDR` | 隧道对端地址（仅探活） | 空=不配置 |
| `COALRAG_TASK_RETENTION_DAYS` | 已结束任务的保留天数 | `7` |
| `COALRAG_QA_LOG_DAYS` | 问答留痕保留天数（`qa_log` 只增不删） | `90` |
| `COALRAG_AUDIT_LOG_DAYS` | 审计日志保留天数 | `180` |
| `COALRAG_BOOTSTRAP` | 空库是否写演示数据 | `true` |

## 切换到矿内真实模型

代码只依赖 OpenAI 兼容接口，切换不需要改业务代码：

```jsonc
// configs/config.json
"llm":   { "provider": "openai", "base_url": "http://10.0.0.21:8000/v1", "model": "deepseek-r1:14b", ... },
"embed": { "provider": "openai", "base_url": "http://10.0.0.21:8000/v1", "model": "bge-m3", "dim": 1024, ... }
```

两件必须做的事：

1. **维度一致性**：启动时比对 `system_meta.embed_dim`，不一致直接拒绝启动。换模型换了维度就要重建向量（删除 `doc_embedding` 里的记录，对已发布文档重新投递 `doc_embed` 任务）。
2. **阈值重标定**：`min_score` 与嵌入模型强绑定。桩实现的标定区间是 `(0.076, 0.085)`，取 0.08；换成 bge-m3 后跑
   `go run ./cmd/tune -doc testdata\煤矿安全规程节选.docx`，按输出的“建议阈值区间”改 `retrieval.min_score`，再用 `POST /api/v1/eval/qa` 复核。

## 桩实现的边界

没部署离线模型时，`stub` 提供确定性替身，走的是与真实模型完全相同的提示词与后处理路径：

- `stub-chat`：解析提示词里的【片段N】，按首个片段作答并给出 `依据：[第X条]`；没有片段时返回兜底话术。
- `stub-embed`：带符号哈希的字符 2/3-gram 向量（1024 维，L2 归一化）。保留字面相似性，不是语义嵌入。
- 因此**评测集分数只反映链路正确性**（切分、检索、引用校验、兜底、留痕），不代表真实模型的语义能力。

## 验收与留档

- `docs/smoke-output.txt`：最近一次后端端到端自测输出，103 项全部通过。
- `docs/ui-verification.txt` + `docs/screenshots/`：浏览器回归输出（39 项断言）与员工端/后台截图。
- `docs/IMPLEMENTATION.md`：S0–S7 与前端每项的实现位置、判据证据。
- `docs/API.md`：接口清单（方法、权限、数据范围）。
- `docs/UI.md`：前端页面清单、构建方式、设计约定。
- `storage/eval/*.json`：检索命中率与问答评测报告（每次评测自动落盘）。

## 备份与恢复

```powershell
# 备份：数据库 dump + storage 目录 + manifest（含 sha256），默认保留最近 7 份
powershell -ExecutionPolicy Bypass -File scripts\backup.ps1
powershell -ExecutionPolicy Bypass -File scripts\backup.ps1 -Out D:\backups -Keep 14

# 恢复：会先删库再重建，交互确认；加 -Force 用于无人值守，-RestoreStorage 一并还原附件
powershell -ExecutionPolicy Bypass -File scripts\restore.ps1 -From backups\coalrag-20261112-142323
```

恢复前必须停掉服务（它占着数据库连接）。`restore.ps1` 会校验 manifest 里的 sha256，损坏的备份直接拒绝。建议把 `backups` 放到另一块盘或摆渡目录，别和 `storage` 同盘。

后台任务表按 `task.retention_days`（默认 7 天）自动清理已结束任务，不需要手工 truncate；考试超过时长未交卷的记录每 5 分钟扫描一次，自动置为超时并按 0 分计入统计。

## 数据模型

一期 14 张：`org_dept` `org_user` `role_policy` `doc` `doc_chunk` `qa_log` `learn_item` `learn_record`
`quiz_question` `exam_paper` `exam_record` `broadcast_task` `broadcast_record` `audit_log`；
二期 4 张：`issue` `issue_flow` `issue_attachment` `issue_cate`；
基础设施 3 张：`task`（任务表，替代 Redis 队列）、`doc_embedding`（向量持久层）、`system_meta`。

约束落实：`issue_flow` 只插不改（代码中无 UPDATE 该表）；状态迁移在事务内 `select ... for update` 后比对状态，冲突返回 409；`doc_chunk.clause_no` 可反查原文段落。

## 检索策略

默认混合检索：向量召回 + BM25 词法召回，用 RRF 融合排序。词法侧按「ASCII 词 + 汉字二元组」切词，条款号、百分比、数字能整体命中；阈值仍按向量相似度判断，词法只在“至少命中两个查询词且相对分接近最高”时放行，避免常用词把无关条款带进上下文。

| 环境变量 | 含义 | 默认 |
| --- | --- | --- |
| `COALRAG_HYBRID` | 是否启用混合检索 | `true` |
| `COALRAG_LEXICAL_MIN` | 词法放行的相对分门槛 | `0.9` |

关掉后回到纯向量。当前语料（21 条条款、桩嵌入）两种模式评测都是 15/15；混合检索的价值在真实嵌入 + 大语料下才体现，接模型后请重跑评测再决定是否常开。

## 运维接口

- `GET /metrics`：Prometheus 文本格式（请求量、问答回答/兜底数、文档与切片数、任务积压、未闭环问题）。
- `GET /api/v1/audit/export`：审计日志 CSV，筛选条件与列表一致。
- `scripts/check.ps1`：一键 `gofmt` 检查 + `go vet` + 单元测试 + 构建；加 `-Smoke` 再跑端到端。

## 文档版本

发布文档时自动保存切片快照（`doc_version`），可查看历史、对比条款差异、回滚。回滚不删除历史，而是生成新版本并重建向量；后台「文档与条款 → 切片 → 版本」里操作。

## 小程序上线（备案后）

备案只是第一步，把小程序接到矿内服务还需要：**已备案域名、证书、一台公网服务器、矿内出站**。
完整步骤见 `deploy/README.md`，这里只列要点：

1. 域名 A 记录指向云端；证书和私钥**只放矿内**。
2. 云端 nginx 按 SNI 做四层转发（`deploy/nginx/cloud-stream.conf`），不做 TLS 终结。
3. 矿内 frpc 主动连云端，把本机 8443（`deploy/nginx/mine-tls.conf`）映射出去。
4. 小程序后台配三处域名：request、业务域名（web-view）、uploadFile/downloadFile。
5. 业务域名校验文件放 `web/public/<文件名>.txt`，重建前端与服务端后即可访问（缺失时返回 404，不会回退成首页）。
6. 外网自检：

```powershell
powershell -ExecutionPolicy Bypass -File scripts\check-public.ps1 -Domain rag.example.com -VerifyFile abc123.txt
```

自检覆盖 DNS、TLS 证书剩余天数、`/healthz`、首页、校验文件类型。

## 小程序登录与网络准入

miniprogram/ 是微信小程序外壳：登录后直接用 web-view 打开现有 H5，用户不需要理解网络。

| 接口 | 作用 |
| --- | --- |
| POST /api/v1/auth/wx-login | 小程序 code 换 openid；已绑定直接发会话，未绑定返回绑定票据 |
| POST /api/v1/auth/wx-bind | 首次绑定工号+口令，写 wx_bind 后发会话 |
| POST /api/v1/auth/ticket | 一次性票据换会话（web-view 打开 H5 时用，2 分钟、只能用一次） |

配置：COALRAG_WX_APPID、COALRAG_WX_SECRET；留空为桩模式（openid=stub-<code>），本地可跑通全流程。

**网络准入**：COALRAG_ALLOWED_CIDRS（逗号分隔）非空时，/api/v1 只对名单内网段开放，/healthz、/metrics 豁免。例如：

`powershell
 = 10.20.0.0/16,203.0.113.7/32
`

未连公司网络的请求返回 403。注意 4G 出口 IP 会漂移，白名单只适合“固定专线出口”或“VPN 网关”场景；要真正限制到网络，还是发 VPDN 专网卡最省事（用户插卡即用）。

## 上传去重

同一份文件按内容 sha256 去重：重复上传直接复用已有文档（解析失败的除外），不会生成多份副本抢检索排名。后台上传对话框里的“强制重新入库”开关（orce=true）可以绕过；文件名带内容哈希前缀，强制上传也不会互相覆盖。

检索侧还有一层兜底：正文完全相同的条款只保留排名最高的一份，避免历史遗留的重复文档把 topK 占满。

## 压测

`powershell
.\bin\loadtest.exe -users 20 -duration 10s -mix ask,search
.\bin\loadtest.exe -users 50 -duration 30s -out .data\load.json
`

输出 QPS、p50/p95/p99、状态码分布与错误分类。**注意**：默认限流 600 次/分钟/IP，压测会大量返回 429——这本身是限流生效的证据，测吞吐时用 COALRAG_RATE_PER_MINUTE 临时调高。

本机基线（桩模型 + 内存向量，非真实推理）：

| 并发 | QPS | p50 | p95 | p99 | 错误 |
| --- | --- | --- | --- | --- | --- |
| 20 | 1078 | 18ms | 22ms | 24ms | 0 |
| 50 | 1032 | 28ms | 55ms | 91ms | 0 |

接真实 14B 模型后瓶颈在推理，这组数字只能说明“链路本身不慢”。

## 探活与告警

方案允许用探活脚本替代监控平台，scripts\watch.ps1 就是它：

`powershell
powershell -ExecutionPolicy Bypass -File scripts\watch.ps1 -IntervalSeconds 60
powershell -ExecutionPolicy Bypass -File scripts\watch.ps1 -Once -Webhook https://example/hook
`

每轮检查 /healthz 的 db/llm/tunnel、以及 /metrics 的任务失败数与超时问题数；异常写 data\watch.log 并可选 POST 到企业微信/钉钉 webhook。阈值 -MaxFailedTasks、-MaxOverdueIssues 按环境调，退出码 0/1 可直接给计划任务用。

## 定时备份

`powershell
# 每天 02:30 备份，保留 7 份
powershell -ExecutionPolicy Bypass -File scripts\install-backup-task.ps1 -Time 02:30 -Keep 7
powershell -ExecutionPolicy Bypass -File scripts\install-backup-task.ps1 -DryRun   # 只看命令
powershell -ExecutionPolicy Bypass -File scripts\install-backup-task.ps1 -Remove   # 删除任务
`

## 账号被锁 / 忘记口令
口令**连续**错 5 次才锁定 15 分钟，单次输错不锁——登录接口会回“还可以尝试 N 次”。锁定后：

- 管理员在后台「组织与权限 → 用户」点「解锁」；
- 管理员自己也被锁时，直接在服务器上执行（直连数据库，不依赖服务是否在跑）：

```powershell
.\bin\coalrag-userctl.exe list                          # 查看账号与锁定状态
.\bin\coalrag-userctl.exe unlock -u admin               # 解锁并清零失败计数
.\bin\coalrag-userctl.exe reset -u admin -p 'NewPass@123'  # 重置口令（同时解锁）
```

`COALRAG_DSN` 可覆盖连接串；忘记口令时用 `reset` 即可，不需要停服务。

## 已知限制

- 前端为内部工具定位：Element Plus 全量引入（后台 chunk 约 950KB，gzip 后 310KB），内网首屏可接受；需要进一步瘦身可换按需引入。
- 员工端未做 PWA/离线缓存，井下无网时不可用；如需离线上岗学习，要另做资源预缓存。
- `pgvector` 实现按配置可启用，但本机 PostgreSQL 未安装该扩展，**未在本地验证**；默认内存实现已覆盖同一套检索用例。
- 检索索引常驻内存（启动从 `doc_embedding` 装载），数十万切片量级需要换 pgvector 或独立向量库。
- 导出为 CSV（Excel 可直接打开），未做 xlsx。
- 文档解析优先 docx 直解；`.doc/.pdf` 依赖宿主机安装 LibreOffice（未装时接口明确报错，不静默降级）。
- 限流与登录锁定在单实例内存中，多实例部署需换成共享存储。
