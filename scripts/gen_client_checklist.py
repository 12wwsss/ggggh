# 生成「甲方需提供资料清单（对外版）」
import math
import openpyxl
from openpyxl.styles import Font, PatternFill, Alignment, Border, Side
from openpyxl.worksheet.datavalidation import DataValidation
from openpyxl.utils import get_column_letter

OUT = r"D:\rag\docs\甲方资料清单（对外版）.xlsx"

NAVY = "17324D"
BLUE = "2F5D8A"
LIGHT = "EDF2F7"
WHITE = "FFFFFF"
GREEN = "E6F4EA"
AMBER = "FFF7E0"
GREY = "F4F5F7"

thin = Side(style="thin", color="D6DCE4")
border = Border(left=thin, right=thin, top=thin, bottom=thin)


def title_block(ws, title, subtitle, ncols):
    ws.merge_cells(start_row=1, start_column=1, end_row=1, end_column=ncols)
    c = ws.cell(row=1, column=1, value=title)
    c.font = Font(name="微软雅黑", size=15, bold=True, color=WHITE)
    c.fill = PatternFill("solid", fgColor=NAVY)
    c.alignment = Alignment(horizontal="left", vertical="center", indent=1)
    ws.row_dimensions[1].height = 32

    ws.merge_cells(start_row=2, start_column=1, end_row=2, end_column=ncols)
    c = ws.cell(row=2, column=1, value=subtitle)
    c.font = Font(name="微软雅黑", size=10, color="44515E")
    c.fill = PatternFill("solid", fgColor=LIGHT)
    c.alignment = Alignment(horizontal="left", vertical="center", wrap_text=True, indent=1)
    ws.row_dimensions[2].height = 30


def header_row(ws, row, headers):
    for i, h in enumerate(headers, start=1):
        c = ws.cell(row=row, column=i, value=h)
        c.font = Font(name="微软雅黑", size=10.5, bold=True, color=WHITE)
        c.fill = PatternFill("solid", fgColor=BLUE)
        c.alignment = Alignment(horizontal="center", vertical="center", wrap_text=True)
        c.border = border
    ws.row_dimensions[row].height = 34


def body_rows(ws, start_row, rows, widths=None):
    for r, row in enumerate(rows, start=start_row):
        for i, v in enumerate(row, start=1):
            c = ws.cell(row=r, column=i, value=v)
            c.font = Font(name="微软雅黑", size=10)
            c.alignment = Alignment(vertical="top", wrap_text=True)
            c.border = border
            if r % 2 == 0:
                c.fill = PatternFill("solid", fgColor="FAFBFD")
        if widths:
            best = 1
            for i, v in enumerate(row):
                per_line = max(4, widths[i] / 1.9)
                lines = sum(max(1, math.ceil(len(p) / per_line)) for p in str(v or "").split("\n"))
                best = max(best, lines)
            ws.row_dimensions[r].height = min(80, max(24, best * 12.5 + 6))


wb = openpyxl.Workbook()

# ---------------- Sheet1 清单 ----------------
ws = wb.active
ws.title = "需甲方提供资料"

headers = ["序号", "类别", "事项", "说明（用途）", "规格 / 要求", "阶段", "建议提供形式", "备注"]
title_block(
    ws,
    "甲方需提供资料清单",
    "用途：平台部署与远程测试 ｜ 阶段说明：开测前＝启动测试必须先具备；上线前＝正式投用前提供；可选＝视需要补充 ｜ 我方联系人：____________",
    len(headers),
)
header_row(ws, 3, headers)

rows = [
    # 一、服务器与网络
    [1, "服务器与网络", "★ GPU 云服务器", "承载平台与 AI 模型的运行环境",
     "GPU 显存 ≥32GB，内存 ≥64GB，16 核，数据盘 ≥300GB，系统 Ubuntu 22.04/24.04",
     "开测前", "提供服务器 IP 与登录方式，或由我方代购", ""],
    [2, "服务器与网络", "★ 公网 IP", "供甲方在任意网络访问平台",
     "固定公网 IPv4 地址（如 47.98.123.45），非内网地址；带宽 ≥5Mbps",
     "开测前", "云平台弹性公网 IP", "如仅有内网 IP，请说明现有映射方式"],
    [3, "服务器与网络", "★ 网络访问开放", "保证浏览器可打开平台页面",
     "对外开放 443 端口（测试阶段可临时用 8080），22 端口仅对运维人员开放",
     "开测前", "云平台安全组配置", ""],
    [4, "服务器与网络", "服务器登录方式", "用于部署、升级与日常维护",
     "SSH 账号与管理员权限；建议密钥登录",
     "开测前", "账号密码或密钥", "如不允许直连，请提供跳板机或 VPN"],
    [5, "服务器与网络", "办公网访问说明", "避免测试时页面打不开",
     "说明办公网络是否限制外网访问、是否有代理或白名单",
     "开测前", "文字说明", ""],
    [6, "服务器与网络", "服务器外网下载", "用于获取模型与依赖",
     "服务器可访问公网下载，或提供离线模型文件",
     "开测前", "放开下载权限或提供文件", "模型约 20GB"],

    # 二、域名与合规
    [7, "域名与合规", "域名（建议）", "通过域名访问，浏览器不提示不安全",
     "一个可管理的二级域名，如 ai.example.com，具备 DNS 解析权限",
     "上线前", "提供域名及解析权限", "测试阶段可先用公网 IP 访问"],
    [8, "域名与合规", "HTTPS 证书（建议）", "保障访问安全，也是小程序接入前提",
     "受信任 CA 签发的证书，可委托我方申请并配置",
     "上线前", "域名解析权限", ""],
    [9, "域名与合规", "ICP 备案", "大陆服务器通过域名对外提供服务的合规要求",
     "备案主体与域名持有人一致，周期约 2–3 周",
     "上线前", "按接入商要求提交材料", "测试阶段使用 IP 访问可暂缓"],

    # 三、账号与人员
    [10, "账号与人员", "★ 测试账号名单", "开通测试账号、分配权限",
     "3–5 个账号：工号、姓名、部门、岗位、角色（管理员/区队长/安全科/培训员/员工）",
     "开测前", "按「人员名单」模板填写", "首次登录需修改初始密码"],
    [11, "账号与人员", "★ 管理员指定", "负责后台管理与账号维护",
     "指定 1 名管理员，提供姓名与工号",
     "开测前", "文字说明", ""],
    [12, "账号与人员", "人员名单（全量）", "批量创建账号",
     "模板：工号、姓名、部门、岗位、角色、手机号",
     "上线前", "按模板提供 Excel", ""],

    # 四、业务资料
    [13, "业务资料", "★ 规程与制度文件", "构建知识库，作为问答依据",
     "《煤矿安全规程》节选、矿安全管理制度、岗位操作规程；docx/pdf/doc，单份 ≤64MB",
     "开测前", "电子文件", "扫描件需可复制文字，图片版需先转文字"],
    [14, "业务资料", "★ 组织架构", "建立部门/区队，确定数据可见范围",
     "部门或区队清单，含上下级关系",
     "开测前", "按模板提供 Excel", ""],
    [15, "业务资料", "岗位清单", "按岗位下发题目、风险点与宣贯",
     "岗位名称、所属部门、人数",
     "上线前", "按模板提供 Excel", ""],
    [16, "业务资料", "岗位风险点", "展示本岗位风险与管控措施",
     "风险点、风险等级、管控措施、依据条款、典型案例",
     "上线前", "Excel 或 Word", "每岗位 3–8 条即可"],
    [17, "业务资料", "培训计划", "新员工带培与必修学习安排",
     "带培周期、阶段目标、师傅人选；必修学习条目与完成期限",
     "上线前", "Excel 或 Word", ""],
    [18, "业务资料", "题库与试卷", "练习与考试",
     "题干、选项、正确答案、解析、依据条款；及格线、时长、考试次数",
     "上线前", "按模板提供 Excel", "建议每岗位 ≥20 题"],
    [19, "业务资料", "隐患分类与时限", "隐患闭环与超时提醒",
     "分类字典（顶板/通风瓦斯/机电/运输/消防等）、受理与整改时限",
     "上线前", "Excel", ""],
    [20, "业务资料", "宣贯与安全文化素材（可选）", "宣贯下发与首页展示",
     "文件、视频、图文、解读要点",
     "可选", "电子文件", "视频建议单条 ≤50MB"],
    [21, "业务资料", "★ 联系方式", "员工端展示与通知",
     "安全科值班电话、通知联系人",
     "开测前", "文字说明", ""],

    # 五、流程确认
    [22, "流程确认", "★ 隐患闭环流程", "配置流转与时限",
     "谁受理、谁整改、谁验收、超时如何计算",
     "开测前", "书面说明或会议确认", ""],
    [23, "流程确认", "★ 数据可见范围", "配置各角色可查看的数据",
     "区队长查看本队、安全科查看全矿等",
     "开测前", "书面确认", ""],
    [24, "流程确认", "转人工值班（可选）", "问答转人工的接收与回复",
     "接收人 1–2 名、回复时限",
     "可选", "文字说明", ""],

    # 六、测试配合
    [25, "测试配合", "测试问题集", "验证问答准确性",
     "10–20 条现场真实问题及期望答案要点",
     "开测前", "按「测试问题集」模板填写", ""],
    [26, "测试配合", "测试时间与配合人", "安排测试窗口与问题处理",
     "测试日期、参与人数、对接人及联系方式",
     "开测前", "文字说明", ""],
]

for r, row in enumerate(rows, start=4):
    for i, v in enumerate(row, start=1):
        c = ws.cell(row=r, column=i, value=v)
        c.font = Font(name="微软雅黑", size=10)
        c.alignment = Alignment(vertical="top", wrap_text=True)
        c.border = border
        if r % 2 == 0:
            c.fill = PatternFill("solid", fgColor="FAFBFD")
    stage = row[5]
    c = ws.cell(row=r, column=6)
    c.alignment = Alignment(horizontal="center", vertical="center", wrap_text=True)
    c.fill = PatternFill("solid", fgColor=GREEN if stage == "开测前" else (AMBER if stage == "上线前" else GREY))
    c.font = Font(name="微软雅黑", size=10, bold=(stage == "开测前"))

widths = [6, 14, 22, 30, 46, 10, 24, 26]
for i, w in enumerate(widths, start=1):
    ws.column_dimensions[get_column_letter(i)].width = w

for r in range(4, 4 + len(rows)):
    best = 1
    for i, v in enumerate(rows[r - 4]):
        per_line = max(4, widths[i] / 1.9)
        best = max(best, sum(max(1, math.ceil(len(p) / per_line)) for p in str(v or "").split("\n")))
    ws.row_dimensions[r].height = min(84, max(26, best * 12.5 + 6))

ws.freeze_panes = "A4"
ws.auto_filter.ref = f"A3:H{3 + len(rows)}"
ws.sheet_view.zoomScale = 90
ws.print_title_rows = "1:3"
ws.page_setup.orientation = "landscape"
ws.page_setup.fitToWidth = 1
ws.sheet_properties.pageSetUpPr.fitToPage = True

# ---------------- Sheet2 人员名单模板 ----------------
ws2 = wb.create_sheet("模板-人员名单")
h2 = ["工号", "姓名", "部门", "岗位", "角色", "手机号", "初始密码", "备注"]
title_block(ws2, "人员名单（模板）", "角色可填：管理员 / 区队长 / 安全科 / 培训员 / 员工；初始密码至少 8 位，首次登录需修改", len(h2))
header_row(ws2, 3, h2)
body_rows(ws2, 4, [
    ["", "", "", "", "管理员", "", "", ""],
    ["", "", "", "", "安全科", "", "", ""],
    ["", "", "", "", "区队长", "", "", ""],
    ["", "", "", "", "员工", "", "", ""],
], [14, 12, 16, 18, 12, 16, 16, 26])
for i, w in enumerate([14, 12, 16, 18, 12, 16, 16, 26], start=1):
    ws2.column_dimensions[get_column_letter(i)].width = w
for r in range(4, 40):
    ws2.row_dimensions[r].height = 22
dv = DataValidation(type="list", formula1='"管理员,区队长,安全科,培训员,员工"', allow_blank=True)
ws2.add_data_validation(dv)
dv.add("E4:E200")
ws2.freeze_panes = "A4"

# ---------------- Sheet3 测试问题集 ----------------
ws3 = wb.create_sheet("模板-测试问题集")
h3 = ["序号", "问题（现场真实问法）", "期望答案要点", "依据条款", "期望引用文件"]
title_block(ws3, "测试问题集（模板）", "请填写 10–20 条，问题越接近现场说法越好，答案写要点即可", len(h3))
header_row(ws3, 3, h3)
body_rows(ws3, 4, [
    [1, "", "", "", ""],
    [2, "", "", "", ""],
    [3, "", "", "", ""],
], [6, 42, 46, 18, 22])
for i, w in enumerate([6, 42, 46, 18, 22], start=1):
    ws3.column_dimensions[get_column_letter(i)].width = w
for r in range(4, 30):
    ws3.row_dimensions[r].height = 30
ws3.freeze_panes = "A4"

# ---------------- Sheet4 岗位与部门 ----------------
ws4 = wb.create_sheet("模板-岗位与部门")
h4 = ["岗位名称", "所属部门", "岗位人数", "主要风险类别", "备注"]
title_block(ws4, "岗位与部门（模板）", "按实际岗位填写；同一岗位跨部门可分行填写", len(h4))
header_row(ws4, 3, h4)
body_rows(ws4, 4, [["", "", "", "", ""] for _ in range(6)], [18, 16, 10, 22, 30])
for i, w in enumerate([18, 16, 10, 22, 30], start=1):
    ws4.column_dimensions[get_column_letter(i)].width = w
for r in range(4, 30):
    ws4.row_dimensions[r].height = 24
ws4.freeze_panes = "A4"

# ---------------- Sheet5 说明 ----------------
ws5 = wb.create_sheet("填写说明")
title_block(ws5, "填写说明", "请按以下顺序准备，资料可分批提供", 2)
notes = [
    ("1. 阶段说明", "开测前：启动测试必须先具备；上线前：正式投用前提供；可选：视需要补充。"),
    ("2. 优先准备", "服务器与网络（1–5）、测试账号（10–11）、规程文件（13）、组织架构（14）、联系方式（21）、闭环流程（22–23）、测试问题集（25）。"),
    ("3. 资料格式", "文档优先提供 docx/pdf；表格按本文件模板填写；扫描件请确保文字可复制。"),
    ("4. 安全与保密", "涉及人员姓名、电话等信息仅用于系统账号与通知，部署完成后可要求删除或脱敏。"),
    ("5. 联系人", "甲方对接人：____________  电话：____________；我方对接人：____________  电话：____________。"),
]
header_row(ws5, 3, ["项目", "说明"])
r = 4
for k, v in notes:
    c1 = ws5.cell(row=r, column=1, value=k)
    c1.font = Font(name="微软雅黑", size=10, bold=True)
    c1.alignment = Alignment(vertical="top", wrap_text=True)
    c1.border = border
    c2 = ws5.cell(row=r, column=2, value=v)
    c2.font = Font(name="微软雅黑", size=10)
    c2.alignment = Alignment(vertical="top", wrap_text=True)
    c2.border = border
    ws5.row_dimensions[r].height = 40
    r += 1
ws5.column_dimensions["A"].width = 18
ws5.column_dimensions["B"].width = 90

wb.save(OUT)
print("saved:", OUT)
