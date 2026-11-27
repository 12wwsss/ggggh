# 生成「甲方需提供的外部条件」Excel 清单
import openpyxl
from openpyxl.styles import Font, PatternFill, Alignment, Border, Side
from openpyxl.worksheet.datavalidation import DataValidation
from openpyxl.utils import get_column_letter

OUT = r"D:\rag\docs\甲方需提供的外部条件清单.xlsx"

NAVY = "1F3A5F"
BLUE = "2E5C8A"
LIGHT = "EAF1F8"
RED = "FDE2E1"
AMBER = "FFF4D6"
GREY = "F2F2F2"
WHITE = "FFFFFF"

thin = Side(style="thin", color="D0D7E2")
border = Border(left=thin, right=thin, top=thin, bottom=thin)

def title_block(ws, title, subtitle, ncols):
    ws.merge_cells(start_row=1, start_column=1, end_row=1, end_column=ncols)
    c = ws.cell(row=1, column=1, value=title)
    c.font = Font(name="微软雅黑", size=16, bold=True, color=WHITE)
    c.fill = PatternFill("solid", fgColor=NAVY)
    c.alignment = Alignment(horizontal="left", vertical="center", indent=1)
    ws.row_dimensions[1].height = 34

    ws.merge_cells(start_row=2, start_column=1, end_row=2, end_column=ncols)
    c = ws.cell(row=2, column=1, value=subtitle)
    c.font = Font(name="微软雅黑", size=10, color="444444")
    c.fill = PatternFill("solid", fgColor=LIGHT)
    c.alignment = Alignment(horizontal="left", vertical="center", indent=1)
    ws.row_dimensions[2].height = 22

def header_row(ws, row, headers):
    for i, h in enumerate(headers, start=1):
        c = ws.cell(row=row, column=i, value=h)
        c.font = Font(name="微软雅黑", size=10.5, bold=True, color=WHITE)
        c.fill = PatternFill("solid", fgColor=BLUE)
        c.alignment = Alignment(horizontal="center", vertical="center", wrap_text=True)
        c.border = border
    ws.row_dimensions[row].height = 36

def body_rows(ws, start_row, rows, fills=None):
    for r, row in enumerate(rows, start=start_row):
        for i, v in enumerate(row, start=1):
            c = ws.cell(row=r, column=i, value=v)
            c.font = Font(name="微软雅黑", size=10)
            c.alignment = Alignment(vertical="top", wrap_text=True)
            c.border = border
            if fills and fills.get(r):
                c.fill = PatternFill("solid", fgColor=fills[r])
            elif r % 2 == 0:
                c.fill = PatternFill("solid", fgColor="FAFBFD")

# ---------------- Sheet1 外部条件 ----------------
wb = openpyxl.Workbook()
ws = wb.active
ws.title = "外部条件清单"

headers = ["序号", "类别", "项目", "具体指什么（含示例）", "规格 / 要求", "为什么需要", "必需性", "提供方", "建议提供时间", "备注 / 常见坑"]
title_block(
    ws,
    "甲方需提供的外部条件清单",
    "版本 2026-11-27 ｜ 用途：云服务器部署 + 甲方远程网页版测试 ｜ 红=必需，黄=建议，灰=后期/可选 ｜ 标 ★ 为开测前必须到位",
    10,
)
header_row(ws, 3, headers)

rows = [
    [1, "基础设施", "★ 云服务器", "一台能跑模型和业务系统的 GPU 云主机（阿里云/腾讯云/华为云等），不是普通办公电脑或虚拟主机", "GPU 显存 ≥32GB（V100 32G / L20 48G）；内存 ≥64GB（最低 32GB + 8GB swap）；16 vCPU；数据盘 ≥300GB SSD；Ubuntu 22.04/24.04", "运行 coalrag 服务 + Qwen2.5-32B 模型 + PostgreSQL", "必需", "甲方 / 甲方云账号", "部署前 1 周", "如甲方已有服务器，需给 SSH 账号与 sudo 权限；显存不够只能降到 14B 模型"],
    [2, "基础设施", "★ 公网 IP", "云主机绑定的一枚固定公网 IPv4 地址（弹性公网 IP），互联网上可直接访问，如 47.98.123.45。注意：192.168.x.x、10.x.x.x 是内网地址，不是公网 IP", "固定不释放、与主机绑定；可被外网 ping/访问；带宽 ≥5Mbps", "① 域名解析 A 记录要指向它；② 甲方在任何网络下访问 http://公网IP:8080 或 https 域名", "必需", "云平台 / 甲方", "部署前", "若服务器只有内网 IP，需要 NAT 端口映射或 VPN，需另外确认；重启后 IP 可能变化，建议弹性 IP"],
    [3, "基础设施", "公网带宽", "云主机出口带宽（决定甲方打开页面、上传文档的速度）", "≥5Mbps（以文字问答为主）；要看视频建议 10Mbps 或视频放对象存储", "保证甲方打开页面、上传文档不卡", "必需", "甲方", "部署前", "按流量计费也能用，但视频/大文件会产生费用"],
    [4, "基础设施", "★ 安全组 / 防火墙", "云平台控制台的「安全组」入方向规则（不是 Windows 防火墙）", "放行 TCP 22（仅限运维/办公 IP）、80、443；其余端口不开放", "让甲方能访问网页，同时不把数据库(5432)、模型(8001/8002)、应用(8080) 暴露到公网", "必需", "甲方 / 云账号", "部署时", "22 端口对 0.0.0.0/0 开放容易被暴力破解，建议限源 IP"],
    [5, "基础设施", "域名（建议）", "一个可管理的域名，如 ai.example.com（example.com 的二级域名）", "有 DNS 解析管理权限，能添加 A 记录指向公网 IP", "HTTPS 证书必须绑定域名；以后小程序 request 合法域名也必须是域名", "建议", "甲方", "部署前", "没有域名可以先 http://IP:8080 测试，但浏览器提示不安全，且无法上小程序"],
    [6, "基础设施", "HTTPS 证书", "由受信任 CA 签发的 SSL 证书（推荐 Let's Encrypt 免费证书，90 天自动续期）", "需要域名解析权限（DNS-01）或 80 端口（HTTP-01）用于签发", "浏览器不再提示「不安全」；小程序强制要求 HTTPS", "建议", "我方申请，需甲方域名权限", "部署时", "自签证书浏览器会拦截，不建议给甲方看"],
    [7, "基础设施", "★ ICP 备案", "在工信部对域名做的网站备案（大陆服务器对外提供网站服务必须）", "备案主体与域名持有人一致；周期约 2–3 周；需营业执照等材料（见「备案材料」页）", "大陆服务器 80/443 对外提供网站需要备案；小程序域名也要求已备案", "必需（大陆）", "甲方主体", "越早越好", "香港/新加坡节点可先免备案测试网页版，但小程序仍需备案域名"],
    [8, "基础设施", "公安备案", "网站备案通过后 30 日内，在全国互联网安全管理服务平台办理", "主体信息、网站信息、负责人信息", "合规要求", "建议", "甲方", "上线后 30 日内", "逾期可能被要求整改"],
    [9, "基础设施", "服务器登录方式", "SSH 账号、密码或密钥，以及 sudo 权限", "22 端口可访问；建议密钥登录", "我方部署、升级、排障", "必需", "甲方", "部署前", "若甲方不允许直连，需要跳板机或 VPN"],
    [10, "基础设施", "内网访问限制说明", "甲方办公网是否限制访问外网、是否有代理/白名单", "提供出口 IP、代理地址、需要放行的域名/IP", "避免甲方测试时打不开页面", "建议", "甲方", "测试前", "有些企业网只放行 443，不放行 8080"],
    [11, "基础设施", "模型下载通道", "服务器能否访问 ModelScope / HuggingFace 下载模型（约 20.6GB）", "可直连下载，或提供离线模型文件（U 盘/内网文件服务器）", "Qwen2.5-32B 权重 + bge-m3 需要下载到服务器", "必需", "甲方", "部署前", "国内服务器建议用 ModelScope 源；下载慢可先下到对象存储再拉取"],

    [12, "测试账号", "★ 测试账号名单", "给甲方测试用的 3–5 个账号：工号、姓名、部门、岗位、角色", "见「模板-人员名单」；角色分管理员/区队长/安全科/员工", "开通账号、分配权限", "必需", "甲方", "测试前 2 天", "首次登录会强制改密码"],
    [13, "测试账号", "管理员指定", "谁负责后台管理（1 人）", "姓名 + 工号", "分配「平台管理员」角色", "必需", "甲方", "测试前", "管理员能看到全矿数据，建议指定安全科或信息中心的人"],
    [14, "测试账号", "测试问题集", "10–20 条甲方真实会问的问题 + 期望答案要点", "见「模板-测试问题集」", "判断问答准确率、是否引用正确条款", "必需", "甲方", "测试前", "问题越接近现场说法越好，不要用书面语改写"],

    [15, "业务数据", "★ 规程 / 制度文件", "正式发布的《煤矿安全规程》节选、矿安全管理制度、岗位操作规程", "docx / pdf / doc，单个 ≤64MB；扫描件需有文字层（或提供 OCR 版）", "知识库与问答依据", "必需", "甲方", "测试前 1 周", "扫描件没有文字层会解析失败，需要 OCR"],
    [16, "业务数据", "事故案例", "本矿或行业事故案例（可脱敏），含经过、原因、教训", "docx / pdf / Word", "案例问答与宣贯素材", "建议", "甲方", "测试前", "涉及人员姓名要脱敏"],
    [17, "业务数据", "FAQ 问答对", "常见问题 + 标准答案（如「瓦斯超限怎么办」）", "CSV（问题,答案）或 JSON", "高频问题直接命中，减少模型幻觉", "可选", "甲方", "测试前", "一条问答一个切片，答案建议带条款号"],
    [18, "业务数据", "★ 组织架构", "部门 / 区队清单，含上下级关系", "Excel：部门名称、上级部门、备注", "建组织、决定数据可见范围", "必需", "甲方", "部署前", "如「综采一队」「综采二队」「通风区」「机电科」"],
    [19, "业务数据", "人员名单", "全量或测试范围人员的工号、姓名、部门、岗位、角色", "见「模板-人员名单」；可先给测试范围", "批量导入账号", "建议", "甲方", "部署后 1 周内", "岗位名称要与岗位清单一致，否则出题/宣贯匹配不上"],
    [20, "业务数据", "岗位清单", "各岗位名称与所属部门、人数", "见「模板-岗位与部门」", "题库、风险点、宣贯按岗位下发", "建议", "甲方", "部署前", "系统里岗位是全矿通用、不绑定部门，需要甲方确认这样是否可以"],
    [21, "业务数据", "岗位风险点", "风险点名称、风险等级、管控措施、依据条款、典型案例", "Excel 或 Word", "小程序「本岗位必知」与风险弹层", "建议", "甲方", "测试前", "每个岗位 3–8 条即可起步"],
    [22, "业务数据", "培训计划", "新员工带培周期、各阶段目标、师傅人选；必修学习条目与完成期限", "Excel：员工/师傅/周期/阶段目标；学习条目/岗位/期限", "带培进度、学习到期提醒", "建议", "甲方", "测试前", "师傅必须是系统里的账号"],
    [23, "业务数据", "题库与试卷", "按岗位的题目：题干、选项、正确答案、解析、依据条款；试卷及格线/时长/次数", "Excel 模板（题干/选项A-D/答案/解析/条款/岗位）", "练习与考试", "建议", "甲方", "测试前", "建议每个岗位 ≥20 题，试卷 10 题"],
    [24, "业务数据", "宣贯内容", "要下发的文件/视频、解读要点、签收要求、截止时间", "文件 + 文字版要点", "宣贯下发与签收", "可选", "甲方", "测试前", "按岗位/区队精准下发时需提供岗位或区队范围"],
    [25, "业务数据", "安全文化素材", "图文、知识卡、短视频（首页文化区展示）", "图片 jpg/png；视频 mp4，建议单条 ≤50MB", "员工首页安全文化", "可选", "甲方", "测试前", "视频建议放对象存储或 CDN"],
    [26, "业务数据", "隐患分类与时限", "隐患分类字典（顶板/通风瓦斯/机电/运输/消防…）、受理/整改/验收时限", "Excel：分类名称、整改时限（小时/天）", "隐患闭环、超时提醒、报表按分类统计", "建议", "甲方", "测试前", "时限用于计算「超时」标记"],
    [27, "业务数据", "★ 联系方式", "安全科值班电话、通知联系人姓名/电话", "文字：姓名 + 电话", "「安全科电话」入口、转人工通知", "必需", "甲方", "部署前", "会显示在员工端"],

    [28, "流程确认", "★ 隐患闭环流程", "谁受理、谁整改、谁验收、谁闭环、超时怎么算", "书面说明或会议确认", "配置角色、时限与流转校验", "必需", "甲方", "测试前", "系统默认：安全科受理 → 责任人整改 → 安全科验收 → 闭环"],
    [29, "流程确认", "转人工值班安排", "谁接收「转人工」、多久内回复", "1–2 人 + 回复时限", "问答转人工（安全科角色收消息）", "建议", "甲方", "测试前", "没有值班安排时，工单会一直 pending"],
    [30, "流程确认", "培训员 / 师傅职责", "谁能建带培计划、确认阶段、写月度小结", "确认角色分配", "带培权限与页面", "建议", "甲方", "测试前", "系统里培训员是全矿范围，师傅只看自己带的人"],
    [31, "流程确认", "数据可见范围", "区队长看本队？安全科看全矿？培训员看哪些", "书面确认", "权限策略（role_policy）", "必需", "甲方", "部署前", "影响报表、问题、带培的数据范围"],

    [32, "合规授权", "等保要求", "是否要求等保三级、是否要测评报告", "说明等级与时间要求", "影响日志留存、审计、测评周期", "建议", "甲方", "立项时", "等保测评由第三方机构做，周期 1–3 个月"],
    [33, "合规授权", "素材授权", "现场照片的肖像权、视频版权、事故案例脱敏要求", "授权书或书面说明", "合规使用培训素材", "建议", "甲方", "使用前", "涉及工人人脸的照片要脱敏或取得授权"],
    [34, "合规授权", "备份位置", "是否允许备份放对象存储 / 异地", "说明允许的位置", "数据库与上传文件备份", "建议", "甲方", "部署前", "默认备份在服务器本地 /opt/coalrag/backups"],
    [35, "合规授权", "日志留存时长", "审计日志、问答留痕保留多久", "默认 180 天；等保可能要求 6 个月", "合规与追溯", "建议", "甲方", "部署前", "系统有 log_retention 定时清理，可调"],

    [36, "小程序（后期）", "小程序 AppID", "企业主体的正式小程序 AppID", "非测试号（测试号不能加插件、不能配服务器域名）", "真机测试与正式发布", "后期", "甲方", "上线前", "当前阶段只做网页版，小程序代码保持更新但不测试"],
    [37, "小程序（后期）", "小程序后台权限", "「开发管理 → 开发设置 → 服务器域名」配置权限", "管理员微信扫码", "把 https 域名加入 request 合法域名", "后期", "甲方", "上线前", "没有这一步，小程序真机请求会被微信拦截"],
    [38, "小程序（后期）", "微信认证", "小程序认证 / 年审（如需插件、支付等能力）", "按微信要求", "能力开通", "可选", "甲方", "上线前", "认证需要企业主体与对公账户"],
]

fills = {}
for i, row in enumerate(rows, start=4):
    need = row[6]
    if need.startswith("必需"):
        fills[i] = RED
    elif need.startswith("建议"):
        fills[i] = AMBER
    elif need.startswith("后期") or need.startswith("可选"):
        fills[i] = GREY

body_rows(ws, 4, rows, fills)

widths = [6, 12, 20, 52, 40, 34, 10, 16, 14, 34]
for i, w in enumerate(widths, start=1):
    ws.column_dimensions[get_column_letter(i)].width = w

import math
def auto_height(ws, start_row, rows, widths, line=12.5, pad=6, cap=80):
    for idx, row in enumerate(rows):
        best = 1
        for col, val in enumerate(row):
            text = str(val or '')
            # 列宽单位约等于 1 个半角字符；中文按 2 个估算
            per_line = max(4, widths[col] / 1.9)
            lines = 0
            for part in text.split('\n'):
                lines += max(1, math.ceil(len(part) / per_line))
            best = max(best, lines)
        ws.row_dimensions[start_row + idx].height = min(cap, best * line + pad)

auto_height(ws, 4, rows, widths)

ws.freeze_panes = "A4"
ws.auto_filter.ref = f"A3:J{3 + len(rows)}"
ws.sheet_view.zoomScale = 90
ws.print_title_rows = "1:3"
ws.page_setup.orientation = "landscape"
ws.page_setup.fitToWidth = 1
ws.sheet_properties.pageSetUpPr.fitToPage = True

# ---------------- Sheet2 备案材料 ----------------
ws2 = wb.create_sheet("备案材料")
h2 = ["序号", "材料", "具体说明", "形式", "份数", "备注"]
title_block(ws2, "ICP / 公安备案材料清单", "以云厂商接入商要求为准，以下为常见清单", len(h2))
header_row(ws2, 3, h2)

rows2 = [
    [1, "营业执照", "副本扫描件，经营范围包含相关内容", "彩色扫描件/清晰照片", "1", "加盖公章"],
    [2, "法人身份证", "正反面", "彩色扫描件", "1", "有效期内"],
    [3, "网站负责人身份证", "正反面；非法人时需授权书", "彩色扫描件", "1", "负责人需参与核验"],
    [4, "域名证书", "域名注册商后台下载", "PDF / 图片", "1", "域名持有人须与备案主体一致"],
    [5, "真实性核验单", "接入商提供模板", "打印签字盖章", "按接入商要求", "部分省份支持 App 在线核验"],
    [6, "幕布照 / 视频核验", "负责人站在指定幕布前拍照或录视频", "照片 / 视频", "按接入商要求", "看云厂商具体要求"],
    [7, "授权书", "法人授权网站负责人办理", "盖章扫描件", "1", "负责人非法人时需要"],
    [8, "前置审批", "如涉及新闻、出版、教育、医疗等需前置审批", "批复文件", "按需", "一般企业网站不需要"],
    [9, "公安备案信息", "网站名称、域名、IP、负责人", "在线填报", "1", "ICP 通过后 30 日内"],
]
body_rows(ws2, 4, rows2)
for i, w in enumerate([6, 20, 46, 22, 14, 34], start=1):
    ws2.column_dimensions[get_column_letter(i)].width = w
for r in range(4, 4 + len(rows2)):
    ws2.row_dimensions[r].height = 30
ws2.freeze_panes = "A4"

# ---------------- Sheet3 人员名单模板 ----------------
ws3 = wb.create_sheet("模板-人员名单")
h3 = ["工号", "姓名", "部门", "岗位", "角色", "手机号", "初始口令", "备注"]
title_block(ws3, "人员名单模板（甲方填写）", "角色可填：平台管理员 / 区队长 / 安全科 / 培训员 / 员工；初始口令至少 8 位，首次登录强制修改", len(h3))
header_row(ws3, 3, h3)
rows3 = [
    ["admin01", "张建国", "安全科", "安全科长", "平台管理员", "13800000001", "Coal@123456", "示例，请替换"],
    ["safety01", "李明", "安全科", "安全员", "安全科", "13800000002", "Coal@123456", "示例，请替换"],
    ["leader01", "王海涛", "综采一队", "队长", "区队长", "13800000003", "Coal@123456", "示例，请替换"],
    ["trainer01", "赵丽", "安全科", "培训员", "培训员", "13800000004", "Coal@123456", "示例，请替换"],
    ["worker01", "陈刚", "综采一队", "采煤机司机", "员工", "13800000005", "Coal@123456", "示例，请替换"],
]
body_rows(ws3, 4, rows3)
for i, w in enumerate([14, 12, 16, 18, 14, 16, 16, 26], start=1):
    ws3.column_dimensions[get_column_letter(i)].width = w
for r in range(4, 40):
    ws3.row_dimensions[r].height = 22
dv = DataValidation(type="list", formula1='"平台管理员,区队长,安全科,培训员,员工"', allow_blank=True)
ws3.add_data_validation(dv)
dv.add("E4:E200")
ws3.freeze_panes = "A4"

# ---------------- Sheet4 测试问题集 ----------------
ws4 = wb.create_sheet("模板-测试问题集")
h4 = ["序号", "问题（甲方真实问法）", "期望答案要点", "依据条款", "期望引用文档", "备注"]
title_block(ws4, "测试问题集模板（甲方填写 10–20 条）", "问题越接近现场说法越好；期望答案写要点即可，不用写全文", len(h4))
header_row(ws4, 3, h4)
rows4 = [
    [1, "瓦斯浓度到 1.5% 了咋办？", "停止工作、切断电源、撤出人员、报告矿调度室", "第一百三十六条", "煤矿安全规程", "示例"],
    [2, "进入作业地点前先干啥？", "敲帮问顶，确认顶板与支护安全；发现离层掉渣立即撤人", "顶板管理相关条款", "煤矿安全规程", "示例"],
    [3, "斜巷行车时人能走吗？", "必须走人行道，严格执行「行车不行人、行人不行车」", "运输相关条款", "煤矿安全规程", "示例"],
]
body_rows(ws4, 4, rows4)
for i, w in enumerate([6, 40, 46, 18, 20, 20], start=1):
    ws4.column_dimensions[get_column_letter(i)].width = w
for r in range(4, 30):
    ws4.row_dimensions[r].height = 26
ws4.freeze_panes = "A4"

# ---------------- Sheet5 岗位与部门 ----------------
ws5 = wb.create_sheet("模板-岗位与部门")
h5 = ["岗位名称", "所属部门", "岗位人数", "主要风险类别", "备注"]
title_block(ws5, "岗位与部门模板（甲方填写）", "系统里岗位目前是全矿通用、不绑定部门；如要求「岗位仅限本部门」，请在此表注明", len(h5))
header_row(ws5, 3, h5)
rows5 = [
    ["采煤机司机", "综采一队", 12, "顶板、机电", "示例"],
    ["支架工", "综采一队", 18, "顶板、支护", "示例"],
    ["瓦斯检查工", "通风区", 9, "通风瓦斯", "示例"],
    ["主提升司机", "机电科", 6, "机电、运输", "示例"],
]
body_rows(ws5, 4, rows5)
for i, w in enumerate([18, 16, 10, 22, 30], start=1):
    ws5.column_dimensions[get_column_letter(i)].width = w
for r in range(4, 30):
    ws5.row_dimensions[r].height = 24
ws5.freeze_panes = "A4"

wb.save(OUT)
print("saved:", OUT)
