# 实现对照表（S0–S7）

方案里的工作项、落地位置与判据证据。S8/S9/S10 本轮不做，列在文末。

## S0 环境探针

- 入口：`cmd/envprobe`
- 采集：主机与 OS、CPU 架构与核数、glibc、内存、磁盘、GPU 与驱动、数据库版本与可用扩展、模型接口连通性、出站能力。
- 输出：`env-report.txt`，每项带 `[OK]` 或 `[未知]`。
- 本机结果（`docs/env-report.txt` 由 `go run ./cmd/envprobe` 生成）：PostgreSQL 18.6 可用，扩展只有 `pg_trgm`（**无 pgvector**，因此默认走内存实现）；`nvidia-smi` 可见 GTX 1660 SUPER 6GB；模型端口 8000 未开放 → 标 `[未知]`，与“大模型离线部署暂不做”一致。

## S1 服务骨架

| 要求 | 实现 |
| --- | --- |
| 配置加载 | `internal/config`：`configs/config.json` + 环境变量覆盖；时长字段支持 `"90s"` 字符串 |
| 结构化日志 | `internal/logx`（slog JSON），每个请求带 `trace_id`（`X-Request-Id` 透传） |
| 数据库连接 | `internal/db.Open`：pgxpool，启动即探活 |
| 迁移执行器 | `internal/db.Migrate`：SQL 用 `go:embed` 打进二进制，按文件名顺序、每文件一个事务、`schema_migration` 去重 |
| `/healthz` | 返回 `db` / `llm` / `tunnel` 三项状态，任一 down 返回 503 |

判据证据：空库启动自动建表（日志 `数据库迁移完成 versions=["0001_schema","0002_seed"]`）；`/healthz` 返回三项（自测 S1 的 4 条断言）。

## S2 登录、JWT、RBAC、数据范围

- 口令：bcrypt（`internal/auth`）。
- 令牌：HS256，8 小时，`issuer=coalrag`，校验算法白名单。
- RBAC：`role_policy` 表（role×resource×action→scope），启动装载为判定表，改权限后热更新；中间件 `require(resource, action)` 把 scope 写进上下文。
- 数据范围：`all|dept|self`，在查询层生效（文档、问答留痕、学习记录、考试、宣贯、问题）。
- 登录保护：连续失败 5 次锁定 15 分钟（`failed_attempts`/`locked_until`），单次失败不锁定并提示剩余次数；管理员可解锁，管理员自己被锁时用 `cmd/userctl` 直连数据库恢复。

判据证据：自测 S2 —— 三类账号登录成功；员工访问 `/users` 得 403；单次输错后正确口令仍可登录；连续错到阈值返回 423；管理员解锁后恢复登录。

## S3 上传、异步解析、条款切分、入库、审核发布

- 上传：`POST /docs`，保存到 `storage/uploads`，登记 `doc`（状态 `parsing`）并投递 `doc_parse` 任务。
- 解析：`internal/parser`
  - `Docx` 直解（zip + `word/document.xml`，无外部依赖）；
  - `LibreOffice` 子进程（`.doc/.pdf` 需要，未安装时不静默降级，直接报错）；
  - `Text`（txt/md）。
- 切分：`parser.Split` 按“第X章/第X节/第X条”结构化；目录行（点线引导）、页码行丢弃；超长条款按句读切段并共用条款号（`part` 递增）；无“第X条”结构时返回错误，避免把非规程文档发布成可检索资料。
- 事务：解析在内存完成后**一次性**写入（`store.ReplaceChunks` 先删后插，单事务）；失败整体回滚并把文档置 `failed`，不留脏切片。
- 审核发布：管理员查看切片 → `publish`（状态机 `pending_review → published`，重复发布 409）→ 投递 `doc_embed`。

判据证据：自测 S3 —— 上传《煤矿安全规程（节选）》切出 21 条、22 个切片，关键条款号正确；员工发布 403；重复发布 409。

## S4 向量检索

- 接口：`vector.Store{Upsert, Search, DeleteByDoc, Reload, Size}`。
- 默认实现：内存索引，启动从 `doc_embedding` 装载，写入即落库，重启不丢。
- 备选实现：pgvector（`<=>` 余弦距离），按 `vector.kind=pgvector` 启用；本机无扩展，未验证。
- 维度校验：`system_meta.embed_dim` 与配置不一致时**拒绝启动**。
- 嵌入文本：章节 + 条款号 + 正文，避免条文脱离语境。
- 混合检索：BM25（汉字二元组 + ASCII 词）与向量召回各取候选，RRF 融合；词法只在“命中至少两个查询词且相对分接近最高”时放行低向量分切片，避免常用词污染上下文。评测口径与纯向量一致，可用 COALRAG_HYBRID=false 对比。

判据证据：`POST /eval/retrieval`（方案要求的 20 题，topK=6）命中率 1.00、MRR 1.00，报告落盘 `storage/eval/retrieval-*.json`。

## S5 问答链路

顺序：检索 → 阈值判断（`min_score`）→ 上下文拼装（受 `max_context_chars` 限制）→ 模型调用 → 引用校验 → 兜底 → 留痕。

- 引用校验：从回答中抽取 `第X条`，只保留检索结果里真实存在的条款号；一个都校验不过就判为“无依据”，改用兜底话术并清空引用。
- 兜底：`资料中未找到明确依据，建议查阅《煤矿安全规程》原文或向通风管理部门核实。`
- 留痕：问题、回答、引用（含 `chunk_id` 可反查原文）、耗时、模型名、IP、是否兜底。

判据证据：自测 S5 —— 可回答问题带正确条款号且 `/chunks/:id/text` 原文一致；无依据问题走兜底；`POST /eval/qa` 14 题准确率 1.00、编造条款 0 条。

## S6 学习、小测、宣贯、考试、看板

| 能力 | 实现要点 |
| --- | --- |
| 岗位学习清单 | `learn_item` 按岗位/部门/状态过滤；员工只见已发布 |
| 学习记录 | `learn_record` 唯一约束 `(item_id,user_id)`；进度只增不减，完成后不再回退 |
| 小测 | 按条目取题，员工端不下发答案；提交即判分并返回逐题解析 |
| 宣贯 | `broadcast_task` 发布时按部门批量生成 `broadcast_record`；签收幂等；逾期未签收由 worker 扫描 |
| 在线考试 | 试卷发布 → 开始（幂等）→ 交卷判分；重复交卷 409；员工端与开始接口都不含答案 |
| 考试次数 | `exam_paper.max_attempts`（1–10，创建时设、发布时还可改）；每次开始生成一条 `exam_record`（`attempt_no` 递增），用完返回 409「考试次数已用完」；员工端显示可考/已考/剩余，交卷后可立即「再考一次」；`GET /exam/papers/:id/record` 只查看成绩与答案，不消耗次数 |
| 三指标看板 | 学习完成率 = 已完成记录 /（在册人数×已发布条目）；考试通过率 = 通过/交卷；宣贯签收率 = 已签收/应签收 |
| 导出 | `/learn/export`、`/exam/export` 输出 UTF-8 CSV |

判据证据：自测 S6 —— 学习→小测→宣贯签收→考试完整走通；看板三项可读。

## S7 二期闭环

- 状态机：`reported → accepted → rectified → verified → closed`，另有 `reported → rejected`、`verified` 前可退回 `accepted`。
- 并发保护：事务内 `select ... for update` 比对当前状态，非法迁移返回 409；`UPDATE ... WHERE id=$1 AND status=$2` 二次兜底。
- `issue_flow` 只插不改：全仓没有对该表的 UPDATE 语句。
- 超时扫描：worker 每 5 分钟投递 `issue_overdue_sweep`，把过期未闭环的问题置 `overdue`（不改变状态，只做标记与计数）。
- 附件：`issue_attachment` 记录 sha256，文件落 `storage/issues/<id>/`；`GET /issues/:id/attachments/:aid` 按数据范围校验后下载原文件。

判据证据：自测 S7 —— 跨状态操作 409；上报→受理→整改→验收→闭环后流转记录正好 5 条；附件上传后可原样下载；统计可读。

## 运行期保障

- **考试超时**：`exam_timeout_sweep` 每 5 分钟扫描 `in_progress` 且已超过试卷时长的记录，置 `timeout`、0 分、写入交卷时间；统计口径把 timeout 计入“已交卷未通过”，关掉浏览器不会永远停在“进行中”。
- **任务表保留**：`task_retention` 每天清理超过 `task.retention_days`（默认 7 天）的已结束任务，避免扫描任务每小时新增 36 条把表撑大。
- **日志表保留**：`log_retention` 每天按 `logs.qa_log_days`（默认 90 天）清理 `qa_log`、按 `logs.audit_days`（默认 180 天）清理 `audit_log`。两张表每个请求写一行，压测两小时 `qa_log` 就到 9258 行，不设上限会一直涨。实测：把一行改成 120 天前，任务触发后 `qa_log_removed=1`。
- **系统日志上限**：`/etc/systemd/journald.conf` 设 `SystemMaxUse=1G`、`SystemMaxFileSize=100M`；实测从 3.9 GB 降到 978 MB。`/opt/coalrag/logs/*.log` 交给 logrotate（每周、保留 8 份、压缩）。
- **备份恢复**：`scripts/backup.ps1` 导出 `pg_dump` + `storage` + manifest（sha256），默认保留 7 份；`scripts/restore.ps1` 校验 sha256 后删库重建并还原，支持 `-RestoreStorage`。已用独立库 `coalrag_restore_test` 做还原验证（8 用户 / 9 文档 / 189 切片一致）。

## 前端

方案要求 Vue3 + Vant（H5）与 Element Plus（后台）。两个入口同仓库、同构建：

- 构建：`web` 目录 Vite 多页入口（`index.html` / `admin.html`），产物 `web/dist` 由 `web/embed.go` 的 `go:embed` 打进服务端二进制。
- 托管：Go 侧 `web.Handler` 按 `/admin` 前缀选择 SPA 入口，`/assets/*` 强缓存，未知 `/api/*` 仍返回 JSON 404；前端未构建时返回“前端尚未构建”的明确提示，不静默 404。
- 员工端：登录、首页待办、问规程（引用 chip 可反查原文、有用/没用反馈）、学习与进度、岗位小测、在线考试（倒计时与自动交卷）、宣贯签收、隐患上报与流转跟踪、修改口令。
- 管理端：运行看板（三指标与口径）、文档与条款（上传/切片预览/发布/驳回/重解析/删除）、问答留痕、学习条目与题库、组卷与成绩导出、宣贯下发与签收明细、问题闭环操作、用户/部门/角色权限矩阵、后台任务与审计。
- 验证：`web/scripts/verify-ui.mjs` 用系统 Edge 的 DevTools 协议做真实渲染与交互回归（16 项断言），截图见 `docs/screenshots/`；细节见 `docs/UI.md`。

## 未实现（按需求本轮不做）

| 项 | 说明 |
| --- | --- |
| S8 目标模型接入与显存调优 | 需要矿内 GPU 与模型权重；当前用桩，接口与提示词路径已就绪 |
| S9 信创迁移 | 需目标 OS/数据库；SQL 集中在 `internal/store`，方言层可替换 |
| S10 隧道、云端反代、证书 | 需云端与矿方网络配合；`/healthz` 已留 `tunnel` 探活位（`COALRAG_TUNNEL_ADDR`） |
