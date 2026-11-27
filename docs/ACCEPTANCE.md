# 验收对照（按技术方案逐条）

验证时间：2026-11-26（离线模型已在 Ubuntu 服务器上部署并实测）
命令：`bin\smoke.exe`（146 项）、`web\scripts\verify-ui.mjs`（50 项）、`scripts\check.ps1`；准确率见 `docs/OFFLINE-EVAL.md`，并发与长稳见 `docs/CONCURRENCY.md`

## 一、工作项 S0–S10

| 编号 | 判据 | 结论 | 证据 / 差距 |
| --- | --- | --- | --- |
| S0 | 输出 env-report.txt，每项给结论或标「未知」 | 达成 | `docs/env-report.txt`：主机、CPU、glibc、内存、磁盘、GPU、数据库版本与扩展、模型连通性、出站能力；离线环境模型与部分项标「未知」 |
| S1 | 空库启动自动建表；/healthz 返回 db、llm、tunnel | 达成 | 迁移 0001–0003 自动执行；`/healthz` 三项状态，任一 down 返回 503 |
| S2 | 三类账号可见数据不同；越权返回 403 | 达成 | 员工访问 `/users` 得 403；数据范围 all/dept/self 在查询层生效；登录失败阈值与解锁实测 |
| S3 | 上传规程可按条款切分、后台可见并发布；解析失败不写脏数据 | 达成 | 21 条条款 / 21 个切片；发布后进入检索；上传无条款结构的文档 → 状态 failed、切片 0 条（用例断言） |
| S4 | 20 个测试问题、topK=6 命中比例可统计并留档 | 达成 | 桩嵌入 20/20；真实 bge-m3 **20/20，MRR 1.00**，报告落盘 `docs/eval-real/` 与 `storage/eval/` |
| S5 | 30 个测试问题；可回答的带条款号且原文可打开；无依据的走兜底、无编造条款 | 达成 | 桩模型 0.97（1 条近似误答）；真实 Qwen2.5-14B **30/30，1.00**，22 答 / 8 兜底，错引 0、误答 0、编造 0，平均 0.86 秒/题 |
| S6 | 一个岗位走完学习、小测、宣贯、考试；完成率与成绩可导出 | 达成 | 自测串联学习→小测→宣贯签收→考试交卷；`/learn/export`、`/exam/export` 输出 CSV |
| S7 | 一条问题走完上报到闭环；超时触发提醒；看板计数与明细一致 | 达成 | 上报→受理→整改→验收→闭环，流转 5 条只增不改；并发两个动作 200×1 / 409×1；超时扫描置 overdue 并计入看板；受理/整改/验收/闭环/驳回都会写站内消息（`message` 表，小程序消息页可读）。**差距**：站内信之外没有短信/企业微信推送，超时提醒只落到站内消息与审计日志 |
| S8 | 接入目标离线模型，完成显存与并发调优 | **基本达成** | 已部署 Qwen2.5-14B-Instruct Q5_K_M（双 2080 Ti，`-ngl 99 --tensor-split 11,11 --parallel 8`），评测与并发均有报告。**差距**：方案默认模型是 DeepSeek 蒸馏 14B，实际用了 Qwen2.5，型号需甲方确认；两个真实模型的逐题对比未做（目前是桩 vs 真实） |
| S10-2 | 小程序登录与内网访问（新增需求） | 部分达成 | 微信登录、工号绑定、一次性票据换会话、web-view 外壳已实现并自测；**缺** AppID/Secret、备案域名、证书与小程序审核，公网入口需按 S10 部署 |
| S9 | S3–S7 用例在目标环境全部通过 | 未做 | 需目标 OS/数据库；SQL 集中在 `internal/store`，方言层未验证 |
| S10 | 手机 4G 可访问；云端抓包无正文；连续错误登录锁定 | **部分达成** | 限流、审计、登录锁定已实现并实测；隧道/TLS/云端反代未配置（缺域名、证书、公网服务器），已备 `deploy/nginx/*`、`deploy/frp/*` 配置模板 |

## 二、数据模型要求

| 要求 | 结论 | 证据 |
| --- | --- | --- |
| 条款号可反查原文段落 | 达成 | 引用带 `chunk_id`，`GET /chunks/:id/text` 返回原文；问答页条款框、后台“复制原文”同一数据源 |
| 向量维度与嵌入模型绑定，不一致拒绝启动 | 达成 | 元数据基线 + **行级校验**；实测把一条向量改成 512 维后启动被拒 |
| issue_flow 只插不改 | 达成 | 全仓无 `UPDATE issue_flow`；状态变更 5 次对应 5 条流转记录 |
| 状态迁移受事务保护 | 达成 | 事务内 `SELECT ... FOR UPDATE`；实测并发两个受理请求：一个 200、一个 409 |

## 三、适配层判定

| 位置 | 判定 | 结论 |
| --- | --- | --- |
| 向量检索 | 同一套检索用例在两种实现下结果一致 | **未验证**：pgvector 代码已写，本机 PostgreSQL 无该扩展 |
| 模型调用 | 改配置三项即可切换，业务代码无改动 | **已达成**：`provider/base_url/model` 从桩切到 llama.cpp 的 OpenAI 兼容接口，业务代码零改动 |
| 数据库方言 | 换库只改该包 | 结构上达成（SQL 全在 `internal/store`），**未在达梦/openGauss 验证** |
| 文档解析 | 同一份文档在两种实现下切分结果一致 | **未验证**：服务器未安装 LibreOffice，目前只有 docx 直解一条路径；PDF/.doc 上传会解析失败 |

## 四、实测数据（2026-11-17，真实模型）

```
检索 topK=6 命中率 1.00（20/20）
问答准确率 1.00（30 题，22 答 / 8 兜底，错引 0、误答 0、编造 0，平均 0.86 秒/题）
问答并发 3.1 QPS @ 8 并发（p95 5.5s）；检索 540 QPS
55 分钟长稳：7977 请求全 200，显存零增长，RSS +48KB，无重启
接口自测 122 项全过；浏览器回归 44 项全过
```

## 五、建设方案 V3.0 第一批（2026-11-26）

《煤矿安全员智能体数字人建设方案 V3.0》六项核心功能的第一批补强，逐条有接口自测或界面断言。

| 方案条目 | 结论 | 证据 |
| --- | --- | --- |
| 六（1）首页主动推送安全文化（图文/知识卡） | 达成 | 员工首页有「安全文化」区（最多 3 条 + 全部）；`/culture` 列表与详情；后台可发布/下架；种子 `0032` 保证全新部署有内容 |
| 六（1）文化内容可日常运营 | 达成 | 后台「安全文化内容」页新建/编辑/排序/上下架/删除；草稿员工不可见、发布后可见、员工越权 403 |
| 六（1）短视频 | 达成 | 视频资料页（上传/外链/分类/播放），入口在员工首页「看视频」 |
| 七（二）Word/PDF/FAQ 导入 | 达成 | docx 直解、PDF 走 pdftotext、doc/wps 走 LibreOffice（服务器已装 24.2.7）、CSV/JSON FAQ 一条一片、无条款结构的制度/操作规程/事故案例按段落切分 |
| 六（6）宣贯按岗位精准推送 | 达成 | `broadcast_task.post_ids`，发布时按「区队 ∪ 岗位」生成签收与站内消息；后台可选岗位 |
| 六（6）自动生成解读要点 | 达成 | `/broadcast/draft` 出要点初稿；真实模型出 JSON，桩/离线模型退化为规则提炼并提示人工确认 |
| 七（二）语音提问与朗读 | 部分达成 | 识别/TTS 代码已就绪，但**测试号不能添加插件**：当前 `enableWechatSI=false`，长按语音=录音附件；换正式小程序后按 README 三步开启（加插件、app.json 声明、开关改 true） |
| 五 使用角色（培训员） | 达成 | 新增 `trainer` 角色：可管带培/学习/题库/宣贯查看，后台与批量导入均可建 |
| 六（3）风险点典型案例 | 达成 | `risk_point.case_text` 后台录入，小程序岗位风险弹层展示 |

## 六、方案未要求、但影响交付的缺口

1. **重复上传不去重**：已实现按 sha256 去重（`force=true` 可绕过），检索侧再按正文哈希去重。
2. 无压测数据 → 已补：`docs/CONCURRENCY.md`（问答 3.1 QPS、检索 540 QPS、55 分钟长稳）。
3. 备份只有脚本 → 已补：systemd timer 每日 02:31，已做恢复演练；仍缺**异地存放**。
4. 无 CI → 仍只有本地 `scripts/check.ps1`。
5. **日志表无界** → 已补：`log_retention` 每日清理（qa_log 90 天、audit_log 180 天）。
6. **推送渠道单一**：站内消息已覆盖受理/整改/验收/闭环/驳回/宣贯/带培/学习到期，仍没有短信或企业微信。
7. **真实语料未导入**：知识库当前是 21 条规程示例条款 + 6 条演示文化内容，评测结论不能直接外推到真实规程。
8. **扫描件 PDF**：只有文字层的 PDF 能抽文本（pdftotext），扫描件需要 OCR，当前会明确报「需要 OCR」而不是产出空内容。
9. **数字人与信创**：三期「智能体数字人 / 展厅数字人 / 品牌文化知识库」尚未开工；国产 OS+数据库适配（S9）与等保测评仍未完成。

## 七、建设方案 V3.0 第二批（2026-11-26）

| 方案条目 | 结论 | 证据 |
| --- | --- | --- |
| 六（5）按岗位、区队看问题分布 | 达成 | `issue.post` 上报时写岗位快照；`/reports/issues` 返回 `by_post`；报表页新增「按岗位」表；自测：上报人岗位「采煤机司机」与问题记录一致，分布表包含该岗位 |
| 六（5）定期生成问题分析报表 | 达成 | worker 每天 08:00-08:09 投递 `issue_daily_report`，周一同发周报；汇总新增/闭环/未闭环/超时/平均闭环时长 + 区队/岗位/分类 Top3，以 `report` 类站内消息发给管理员与安全科；`HasTaskToday` 保证一天只发一次 |
| 报表预览 | 达成 | `GET /reports/issue-digest?period=daily|weekly`；自测：日报返回「隐患日报：新增 x / 闭环 y」，正文含未闭环统计；员工访问 403 |
| 七（二）PDF 导入 | 达成 | `parser.Pdf` 调 `pdftotext -enc UTF-8 -nopgbrk`；服务器 pdftotext 24.02；无文字层报「需要 OCR」 |
| 七（二）doc/wps 导入 | 达成 | 服务器已装 LibreOffice 24.2.7（`--no-install-recommends libreoffice-writer`），`soffice --version` 正常 |

## 八、培训管理补齐（2026-11-27）

对照《煤矿安全培训规定》与矿上师带徒制度，补培训员/师傅职责对应的三块能力。

| 项 | 结论 | 证据 |
| --- | --- | --- |
| 培训员可见后台带培 | 达成 | 菜单与新建按钮都放行 `trainer`；浏览器回归「培训员可见带培菜单与新建按钮」 |
| 师傅月度小结/期满总结 | 达成 | `training_note` 表；培训员写小结 200、按计划可查、员工写小结 403；带培展开区有「师傅小结」录入与列表 |
| 一人一档导出 | 达成 | `GET /export/training-archive?user_id=` 返回 xlsx（PK 头），解压校验 7 个 sheet：基本信息/学习记录/考试记录/宣贯签收/错题/上报问题/带培与小结；员工访问 403 |
| 小程序同步 | 达成 | `/api/training` 带 `canWrite`/`notes[]`；`POST /api/training/note` 师傅 200、徒弟 403；远端实测同一份计划里小结即时可见 |
| 师傅视角看带培 | 达成 | `/api/training` 同时返回「我是徒弟的计划」与「我带的人的计划」（`asMentor`/`userName`）；带培页支持多学员切换、师傅可写小结；自测「师傅能在小程序看到自己带的学员计划」，远端 `asMentor:true` 已验证 |
| 师傅确认阶段 | 达成 | `POST /api/training/stage`（师傅/培训员/管理员）；小程序阶段行有「确认完成/撤销完成」；确认后给徒弟发站内消息；自测：师傅确认 200、徒弟看到进度、徒弟操作 403 |
| 删除带培计划 | 达成 | `DELETE /training/plans/:id` 仅管理员可用，阶段与师傅小结级联删除；后台「操作」列有删除按钮（二次确认）；自测：培训员 403、管理员 200、删后 GET 404 |
| 带培进度统计 | 达成 | 列表原来不查阶段，所有计划都显示 0%；改为 SQL 子查询直接返回 `stage_done/stage_total`，陈刚 4/4 显示 100%、赵磊 0/4 |
| 角色下拉包含培训员 | 达成 | 「新建账号 → 角色」原来写死四个角色，漏了培训员；补齐后浏览器回归「新建账号角色下拉有培训员」，远端 Org chunk 同时含「培训员 / trainer」 |

## 九、2026-11-26 增量部署记录（192.168.134.103）

| 项 | 内容 |
| --- | --- |
| 时间 | 2026-11-26 20:47 CST，`systemctl restart coalrag`，服务 active |
| 版本 | `/opt/coalrag/bin/coalrag-server`，sha256 `fb629e22d5768e62345c760181f9afd9dffe4fda7cd649afd4966c0014877de7`（16,240,792 B，`GOOS=linux GOARCH=amd64 CGO_ENABLED=0`） |
| 迁移 | 启动日志 `数据库迁移完成 versions=[0029_plan_batch1 0030_broadcast_posts 0031_content_admin 0032_seed_content]`，`schema_migration` 当前 0032 |
| 备份 | 二进制 `coalrag-server.bak-20261126-204711` / `-204729`；数据库 `/opt/coalrag/backups/pre-batch1-20261126-204717.dump`（670 KB，`pg_restore -l` 校验通过） |
| 验证 | `/api/v1/culture` 返回 3 篇图文、`/api/v1/content` 7 条（含 0032 新增 2 条）；前端 `admin-CNQE-u_D.js` 与 `Content-CC-En3pQ.js` 均 200；浏览器只读巡检 7/7（首页文化区 3 条、文化页 6 卡、详情、后台内容页） |
| 已知问题 | `llama-chat` / `llama-embed` 未启动（8001/8002 拒绝连接），`/healthz` 为 degraded，问答链路暂不可用；检索、学习、宣贯、内容不受影响 |
| 回滚 | `sudo cp /opt/coalrag/bin/coalrag-server.bak-20261126-204711 /opt/coalrag/bin/coalrag-server && sudo systemctl restart coalrag`；如数据库需回退：`sudo -u postgres pg_restore -d coalrag --clean /opt/coalrag/backups/pre-batch1-20261126-204717.dump` |

截图：`docs/screenshots/deploy-employee-home.png`、`deploy-employee-culture.png`、`deploy-employee-culture-detail.png`、`deploy-admin-content.png`。

### 同日晚间：第二批增量部署

| 项 | 内容 |
| --- | --- |
| 版本 | sha256 `c5bac4d965bb45f4019e7afd51d11373e2808f1ca3ea05b2d5c6980cdfb7ca66`（16,269,464 B），22:03 重启 |
| 迁移 | `0033_issue_post`；`issue.post` 列已加，7 条历史问题按上报人岗位回填 |
| 备份 | 二进制 `coalrag-server.bak-20261126-220132`；数据库 `/opt/coalrag/backups/pre-batch2-20261126-220132.dump`（673 KB） |
| 服务器依赖 | 已装 `poppler-utils`（pdftotext 24.02）与 `libreoffice-writer 24.2.7`（`/usr/bin/soffice`） |
| 验证 | `/api/v1/issues` 200；`/reports/issues` 的 `by_post` 三项（采煤机司机 5 / 综采一队队长 1 / 支架工 1）；`/reports/issue-digest?period=daily` 返回「隐患日报」正文；同一份《煤矿安全规程（节选）》转成的 **PDF 与 DOC 各切出 21 条条款**（docx 也是 21 条）；远端后台 `Reports-Cosfck5k.js` 含「按岗位 / 预览日报」 |
| 修复记录 | 首版部署发现两处问题并当日修复：① LibreOffice 以服务账号运行时报 `User installation could not be completed`（HOME 只读）→ 传 `-env:UserInstallation` 到临时目录；② pdftotext 结果先按 8 行合并导致条款号识别不到 → 改为一行一块，交给条款切分 |

### 同晚问答体验修复（代码已推，待小程序重编译）

| 项 | 内容 |
| --- | --- |
| 现象 | 文章详情点「问问 AI」后，聊天页把 `%E7%93%A6%E6%96%AF…` 当问题发出去，检索命中 0，只回「资料中未找到明确依据」 |
| 原因 1 | `wx.navigateTo` 的 query 不会自动解码，`chat.js` 直接把 `options.q` 当问题发送 |
| 修复 1 | `chat.js` 新增 `safeDecode`（最多解两次，兼容已解码与非法编码），并补 node 用例 |
| 原因 2 | 103 上 `doc_embedding` 为 0（21 个切片没有向量），且 `min_score=0.35` 把纯词法命中全部过滤，检索永远为空 |
| 修复 2 | 重新上传并发布《煤矿安全规程（节选）》为 doc 53，`doc_embedding` 0 → 21；检索恢复（第一百三十六条 score 0.68） |
| 原因 3 | `llama-chat` 崩溃重启（restart counter 56）：日志 `failed to fit params to free device memory: n_gpu_layers already set by user to 99, abort`，GPU 同时被 gazebo 占用约 1.6 GB |
| 处理 | 服务端改为「模型不可用 → 返回检索条款 + 前 3 条引用」的降级回答，不再 500/空白；`llama-chat` 的显存参数待定，`llama-embed` 正常 |
| 验证 | `POST /qa/ask`（模型不可用）返回 `answered=false` + 「模型服务暂不可用，先列出检索到的条款…」+ 第一百三十六/一百三十八条引用；本地 8082 以 dead base_url 复现同一行为 |

### 2026-11-27 上午：培训管理补齐上线

| 项 | 内容 |
| --- | --- |
| 版本 | sha256 `f97f7f7d383ff7c963714e5677e9d9eb06e22b3e7edd90dae6bacd2bd687befe`（16,314,520 B），10:59 重启 |
| 迁移 | `0034_training_note`（新建 `training_note` 表） |
| 备份 | `/opt/coalrag/backups/pre-batch3-20261127-105940.dump`（788 KB） |
| 验证 | 建计划 → 写小结 200 → 小结列表可见；导出档案 200、5,155 字节、`PK` 头、7 个 sheet；远端 `Training-H5RM5heE.js` 含「师傅小结」、`Reports-Bvby0d6R.js` 含「training-archive / 培训档案」 |

### 2026-11-27 中午：小程序同步师傅小结

| 项 | 内容 |
| --- | --- |
| 版本 | sha256 `111d4066dda6be78b06f8429a10bf68729d769062df8a536bee68dcda0b377c8`（16,322,712 B） |
| 接口 | `GET /api/training` 返回 `canWrite`/`notes[]`；`POST /api/training/note` 写小结 |
| 远端验证 | worker01 视角 `canWrite:false`、`notes` 随写随见；师傅（管理员）写 200；徒弟写 403；测试计划已清理 |
| 小程序端 | 带培页新增「师傅小结」：可写角色显示表单（类型/月份/正文），所有人可看历史小结；**需在开发者工具重新编译** |

### 2026-11-27 15:10：小程序确认带培阶段

| 项 | 内容 |
| --- | --- |
| 问题 | 师傅只能在小程序看阶段，不能勾完成；后台才有复选框，师傅在井下不方便回办公室 |
| 修复 | `POST /api/training/stage`（师傅/培训员/管理员），带培页每个阶段右下方「确认完成/撤销完成」，确认后给徒弟发 `training` 站内消息 |
| 验证 | smoke 146 项（师傅确认 200、徒弟看到 done、徒弟 403）；远端临时计划实测同一链路后清理 |

### 2026-11-27 14:40：删除计划与进度修正

| 项 | 内容 |
| --- | --- |
| 版本 | sha256 `891a3935ed529d516bfdd41c2f903522304dd8e36e5a893096c2fb8cd5b00b43` |
| 新增 | 管理员可删除带培计划（阶段/小结级联），后台操作列「删除」+ 二次确认 |
| 修复 | 列表接口 `stage_done/stage_total` 恒为 0（没查阶段），后台进度永远 0%；改为 SQL 子查询统计 |
| 数据清理 | 删除赵磊重复的「李四带培计划」（已 1/4 进度）；现存赵磊 0/4 进行中、陈刚 4/4 已完成 |

### 2026-11-27 14:05：师傅视角

| 项 | 内容 |
| --- | --- |
| 问题 | 用户「刘洋」在后台是**带培师傅**（带赵磊），小程序却显示「暂无计划」：`compatTraining` 只查 `training_plan.user_id = 我`，没有查 `mentor_user_id = 我` |
| 修复 | 接口同时返回两种身份的计划（徒弟：自己的；师傅：自己带的人，可多份），带 `asMentor`、`userName`、`deptName`；小程序带培页加「带培对象」picker、头部显示学员 |
| 附带 | 后台带培列表状态列原本显示 active/done，改为 进行中/已完成/已暂停 |
| 验证 | smoke 140 项、UI 49 项全过；远端临时计划实测 `asMentor:true`、`userName`、`canWrite:true`，`Training-BKA8qreO.js` 含「进行中/已完成/师傅小结」 |

### 2026-11-27 11:20：补齐角色下拉

| 项 | 内容 |
| --- | --- |
| 版本 | sha256 `afa018e7de7a8eed6f9a16b7ebc1921f801c78e1f82e9e7ac582bd885f3686ab` |
| 问题 | 组织与权限「新建账号 → 角色」写死 平台管理员/区队长/安全科/员工，没有培训员；`/api/v1/roles` 本来就返回 trainer，角色权限页有，只有这个下拉漏了 |
| 修复 | `Org.vue` 下拉补 `<el-option label="培训员" value="trainer" />` |
| 验证 | 浏览器回归 49 项全过（新增「新建账号角色下拉有培训员」）；远端 `/api/v1/roles` = admin,dept_leader,safety,trainer,employee，`Org-1mzlDfd1.js` 含「培训员 / trainer」 |

截图：`docs/screenshots/deploy-employee-home.png`。
