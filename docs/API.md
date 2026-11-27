# 接口清单

约定：

- 统一前缀 `/api/v1`，除登录外都需要 `Authorization: Bearer <token>`。
- 响应体 `{"code":0,"message":"ok","data":...}`；分页数据为 `{"list":[],"total":0,"page":1,"page_size":20}`。
- 权限列写成 `资源:动作(数据范围)`，范围 `all|dept|self` 由 `role_policy` 表决定，可在线改。
- 越权返回 403；跨状态流转返回 409（不是 400/500）；未登录或令牌失效返回 401；口令错误次数超限返回 423。

## 探活

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/healthz` | 无 | 返回 db、llm、tunnel 三项状态；任一 down 时 HTTP 503 |
| GET | `/readyz` | 无 | 依赖就绪探针 |

## 认证

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| POST | `/auth/login` | 无 | `{username,password}`；单次失败返回 401 并提示剩余次数，连续失败达阈值（默认 5 次）才锁定 15 分钟并返回 423 |
| GET | `/auth/me` | 登录 | 当前用户与权限矩阵 |
| POST | `/auth/password` | 登录 | 修改本人密码，新密码至少 8 位 |

## 组织与权限

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/depts` | dept:read | 部门树 |
| POST | `/depts` | dept:write | 新建部门 |
| PUT | `/depts/:id` | dept:write | 修改部门 |
| GET | `/users` | user:read | 分页；`dept` 范围自动按本部门过滤 |
| POST | `/users` | user:write | 新建账号，初始口令至少 8 位 |
| PUT | `/users/:id` | user:write | 修改资料/角色/状态 |
| POST | `/users/:id/password` | user:write | 重置口令并解锁 |
| POST | `/users/:id/unlock` | user:write | 解除登录锁定 |
| GET | `/roles` | user:read | 角色与权限矩阵 |
| PUT | `/roles/:role` | user:write | 覆盖某个角色的权限项 |

## 文档

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/docs` | doc:read | 管理员可见全部状态，其他角色只看已发布 |
| POST | `/docs` | doc:write | multipart：`file`（≤64MB）、`title`、`category`；入库后异步解析。支持 docx/doc/pdf/txt/md/csv/json（PDF 走 pdftotext，doc/wps 走 LibreOffice）：规程按「第X条」切分，制度/操作规程/事故案例等无条款号文档按小标题与段落切分，FAQ 用 CSV（问题,答案）或 JSON（`[{"q","a"}]`），一条问答一个切片（`FAQ-1`…） |
| GET | `/docs/:id` | doc:read | 文档详情（状态、错误信息、切片数） |
| GET | `/docs/:id/chunks` | doc:read | 切片列表；非管理员只看已发布切片 |
| POST | `/docs/:id/publish` | doc:write | 仅 `pending_review` 可发布，重复发布 409 |
| POST | `/docs/:id/reject` | doc:write | 驳回并记录原因 |
| POST | `/docs/:id/reparse` | doc:write | 重新投递解析任务 |
| DELETE | `/docs/:id` | doc:write | 删除文档、切片、向量与源文件 |
| GET | `/chunks/:id` | doc:read | 单个切片 |
| GET | `/chunks/:id/text` | doc:read | `text/plain` 原文段落，用于引用“原文可打开” |

## 问答

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| POST | `/qa/ask` | 登录 | `{question}`；返回回答、处置动作 `actions[]`（编号行解析结果）、引用（含 chunk_id）、是否兜底、耗时、留痕 id。提问不设 RBAC 写入门槛（员工日常动作） |
| POST | `/qa/search` | qa:read | `{question,top_k,min_score}` 只检索不生成，用于排障 |
| GET | `/qa/logs` | qa:read | 按数据范围查询留痕，可按 `answered=true/false` 过滤 |
| GET | `/qa/logs/:id` | qa:read | 单条留痕（越范围 403） |
| POST | `/qa/logs/:id/feedback` | 登录 | `useful|useless|wrong_clause`；员工只能反馈自己的提问 |

## 评测

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| POST | `/eval/retrieval` | eval:write | `{cases:[{question,expect_clause}],top_k}`；统计命中率与 MRR 并落盘 |
| POST | `/eval/qa` | eval:write | 跑完整问答链路，统计准确率、兜底率、编造条款数 |
| GET | `/eval/reports` | eval:read | 报告列表 |
| GET | `/eval/reports/:name` | eval:read | 读取报告（文件名白名单校验） |

## 学习、小测、宣贯、考试

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET/POST | `/learn/items` | learn:read / write | 学习条目；员工只见已发布 |
| GET/PUT | `/learn/items/:id` | learn:read / write | 详情（含本人学习记录）/ 修改 |
| POST | `/learn/items/:id/progress` | learn:write | `{progress,duration_sec}`；进度只增不减 |
| GET | `/learn/records` | learn:read | 按数据范围查询 |
| GET | `/learn/export` | learn:read | CSV（UTF-8，Excel 直接打开） |
| GET | `/quiz/items/:id/questions` | learn:read | 员工端不下发答案与解析 |
| GET | `/quiz/questions` | learn:read | 题库全量（可选 `?item_id=`），管理端组卷用 |
| POST | `/quiz/questions` | learn:write | 新建题目（`?item_id=` 关联条目） |
| POST | `/quiz/items/:id/submit` | learn:write | 交卷即判分，返回逐题结果与解析 |
| GET/POST | `/exam/papers` | exam:read / write | 试卷列表 / 新建（草稿） |
| GET | `/exam/papers/:id` | exam:read | 试卷与题目（员工端隐藏答案） |
| POST | `/exam/papers/:id/publish` | exam:write | 发布 |
| POST | `/exam/papers/:id/start` | exam:write | 幂等；返回题目、截止时间与剩余秒 |
| POST | `/exam/papers/:id/submit` | exam:write | 判分；重复交卷 409 |
| GET | `/exam/records` | exam:read | 成绩明细（按范围） |
| GET | `/exam/export` | exam:read | CSV 导出 |
| GET/POST | `/broadcast/tasks` | broadcast:read / write | 宣贯任务；`dept_ids`（区队）与 `post_ids`（岗位）都不填表示全员；`publish:true` 时按范围生成签收记录与站内消息 |
| POST | `/broadcast/draft` | broadcast:write | `{title,content}` 生成「解读要点」初稿：`{points[],content,source,model}`；模型不可用时按正文规则提炼并标记 `source=rule` |
| POST | `/broadcast/tasks/:id/publish` | broadcast:write | 草稿发布 |
| GET | `/broadcast/tasks/:id/records` | broadcast:read | 签收明细 |
| POST | `/broadcast/tasks/:id/ack` | broadcast:write | `{ack:true}` 签收 |
| GET | `/broadcast/mine` | broadcast:read | 我的待办宣贯（`pending=true`） |
| DELETE | `/training/plans/:id` | 管理员 | 删除带培计划（阶段与小结级联删除） |
| GET | `/training/plans/:id/notes` | training:read | 带培小结列表（师傅/培训员/本队队长/本人可看） |
| POST | `/training/plans/:id/notes` | training:write | 写师傅月度小结或期满总结：`{kind:monthly|final, period:"2026-11", content}`，至少 10 字 |

## 问题闭环（二期）

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/issues` | issue:read | 按状态/部门/关键字查询，员工只看自己相关 |
| GET | `/issue-cates` | issue:read | 问题分类，供上报表单选择 |
| POST | `/issues` | issue:write | 上报，编号形如 `WT20251111-0001`；服务端按上报人写入 `post` 岗位快照 |
| GET | `/issues/:id` | issue:read | 详情（越范围 403） |
| GET | `/issues/:id/flow` | issue:read | 流转记录（只读） |
| POST | `/issues/:id/accept` | issue:write | `reported → accepted`，可指派与设期限 |
| POST | `/issues/:id/reject` | issue:write | `reported → rejected`，必须填原因 |
| POST | `/issues/:id/rectify` | issue:write | `accepted → rectified`，填整改结果 |
| POST | `/issues/:id/verify` | issue:write | `rectified → verified`；`pass:false` 退回 accepted |
| POST | `/issues/:id/close` | issue:write | `verified → closed` |
| GET/POST | `/issues/:id/attachments` | issue:read / write | 附件列表 / 上传（≤32MB） |
| GET | `/issues/:id/attachments/:aid` | issue:read | 下载附件原文件（按数据范围校验） |
| GET | `/issues/stats` | issue:read | 各状态计数与超时数 |

## 小程序兼容层（`/api/*`，返回 `{ok:true,...}`）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/training` | 当前用户的带培计划：徒弟返回自己的，师傅返回自己带的人（可能多份）。字段含 `asMentor`、`userName`、`canWrite`、`notes[]` |
| POST | `/training/note` | 小程序写师傅小结：`{plan_id,kind:monthly|final,period,content}`，仅师傅/培训员/管理员 |
| POST | `/qa/transfer` | 小程序转人工：`{question,answer,log_id}`，提交给安全科 |
| GET | `/qa/tickets` | 小程序查看自己的转人工与回复 |
| POST | `/training/stage` | 小程序确认/撤销阶段：`{stage_id,done}`，仅师傅/培训员/管理员；确认后给徒弟发站内消息 |
| GET | `/exam/records/:id` | 成绩回看：`detail[]` 每题含 `q`、`o[]`、`a`（正确项下标）、`your`、`picked`、`right`、`e`（解析） |
| POST | `/exam/submit` | 交卷，`result.record_id` 可用于回看 |

## 报表

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/reports/learning` | 管理员/安全科 | 学习档案 |
| GET | `/reports/wrong` | 管理员/安全科 | 错题统计 |
| GET | `/reports/issues` | 管理员/安全科 | 隐患趋势，含 `by_dept`、`by_post`、`status`、环比 |
| GET | `/reports/issue-digest` | 管理员/安全科 | `?period=daily|weekly` 预览隐患日报/周报（与定时推送同一份统计）：`{digest,title,body}` |
| GET | `/export/training-archive` | 管理员/安全科/培训员/区队长（限本队） | `?user_id=` 导出「一人一档」xlsx：基本信息 / 学习记录 / 考试记录 / 宣贯签收 / 错题 / 上报问题 / 带培与小结 共 7 个 sheet |

## 安全文化内容

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/content` | content:read | 管理端列表，含草稿/下架；可按 `kind=article|culture|notice`、`status=draft|published|archived` 过滤 |
| POST | `/content` | content:write | 新建或修改内容；`{id,kind,cat,title,tag,summary,body,date,minutes,ask,sort,status}`，`body` 一行一段 |
| DELETE | `/content/:id` | content:write | 删除内容 |
| GET | `/culture` | content:read | 员工端列表，仅已发布；`kind` 默认 `article`，可带 `cat` |
| GET | `/culture/:id` | content:read | 员工端详情，仅已发布 |

## 运维

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/dashboard` | dashboard:read | 三指标 + 文档/切片/问答/问题计数 |
| GET | `/audit` | audit:read | 审计日志（所有写请求自动记录） |
| GET | `/tasks` | task:read | 后台任务列表 |
| POST | `/tasks/:id/retry` | task:write | 人工重试失败任务 |
