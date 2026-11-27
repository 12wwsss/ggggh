# 生成「甲方资料清单（对外·精简版）」——网页版测试最小集
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


def body_rows(ws, start_row, rows, widths):
    for r, row in enumerate(rows, start=start_row):
        for i, v in enumerate(row, start=1):
            c = ws.cell(row=r, column=i, value=v)
            c.font = Font(name="微软雅黑", size=10)
            c.alignment = Alignment(vertical="top", wrap_text=True)
            c.border = border
            if r % 2 == 0:
                c.fill = PatternFill("solid", fgColor="FAFBFD")
        best = 1
        for i, v in enumerate(row):
            per_line = max(4, widths[i] / 1.9)
            best = max(best, sum(max(1, math.ceil(len(p) / per_line)) for p in str(v or "").split("\n")))
        ws.row_dimensions[r].height = min(80, max(28, best * 12.5 + 8))


wb = openpyxl.Workbook()

# ---------------- Sheet1 清单 ----------------
ws = wb.active
ws.title = "需甲方提供资料"

headers = ["序号", "类别", "事项", "说明（用途）", "规格 / 要求", "建议提供形式", "备注"]
title_block(
    ws,
    "甲方需提供资料清单（网页版测试）",
    "以下 8 项为启动远程测试所需；其余资料（域名备案、题库、岗位风险点、培训计划等）可在测试通过后补充 ｜ 我方联系人：____________",
    len(headers),
)
header_row(ws, 3, headers)

rows = [
    [1, "服务器与网络", "★ GPU 云服务器", "承载平台与 AI 模型的运行环境",
     "GPU 显存 ≥32GB，内存 ≥64GB，16 核，数据盘 ≥300GB，系统 Ubuntu 22.04/24.04",
     "提供服务器 IP 与登录方式，或由我方代购", ""],
    [2, "服务器与网络", "★ 公网 IP 与端口", "供甲方远程访问",
     "固定公网 IPv4 地址（如 47.98.123.45），带宽 ≥5Mbps；开放 443（测试期可临时用 8080），22 端口仅对运维人员开放",
     "云平台弹性公网 IP + 安全组配置", "如仅有内网 IP，请说明现有映射方式"],
    [3, "服务器与网络", "服务器登录方式", "用于部署、升级与日常维护",
     "SSH 账号与管理员权限，建议密钥登录",
     "账号密码或密钥", "如不允许直连，请提供跳板机或 VPN"],
    [4, "账号与人员", "★ 测试账号名单", "开通测试账号、分配权限",
     "3–5 个账号：工号、姓名、部门、岗位、角色（管理员/区队长/安全科/员工）",
     "按「人员名单」模板填写", "首次登录需修改初始密码"],
    [5, "业务资料", "★ 规程与制度文件", "构建知识库，作为问答依据",
     "《煤矿安全规程》节选、矿安全管理制度、岗位操作规程；docx/pdf/doc，单份 ≤64MB",
     "电子文件", "扫描件需可复制文字，图片版需先转文字"],
    [6, "业务资料", "★ 组织架构", "建立部门/区队，确定数据可见范围",
     "部门或区队清单，含上下级关系",
     "按「组织架构」模板填写", ""],
    [7, "业务资料", "★ 安全科联系方式", "员工端展示与通知",
     "安全科值班电话、联系人",
     "文字说明", "会显示在员工端"],
    [8, "流程确认", "★ 数据可见范围", "配置各角色可查看的数据",
     "区队长查看本队、安全科查看全矿等",
     "一句话说明", "未确认时按默认配置"],
]

widths = [6, 14, 22, 30, 52, 26, 26]
for i, w in enumerate(widths, start=1):
    ws.column_dimensions[get_column_letter(i)].width = w
body_rows(ws, 4, rows, widths)
for r in range(4, 4 + len(rows)):
    c = ws.cell(row=r, column=3)
    c.font = Font(name="微软雅黑", size=10, bold=str(c.value).startswith("★"))

ws.freeze_panes = "A4"
ws.auto_filter.ref = f"A3:G{3 + len(rows)}"
ws.sheet_view.zoomScale = 95
ws.print_title_rows = "1:3"
ws.page_setup.orientation = "landscape"
ws.page_setup.fitToWidth = 1
ws.sheet_properties.pageSetUpPr.fitToPage = True

# ---------------- Sheet2 人员名单模板 ----------------
ws2 = wb.create_sheet("人员名单")
h2 = ["工号", "姓名", "部门", "岗位", "角色", "手机号", "初始密码", "备注"]
title_block(ws2, "人员名单（模板）", "每行一人；角色可填：管理员 / 区队长 / 安全科 / 员工；初始密码至少 8 位，首次登录需修改", len(h2))
header_row(ws2, 3, h2)
body_rows(ws2, 4, [
    ["worker01", "张三", "综采一队", "采煤机司机", "员工", "13800000000", "Coal@123456", "示例，请替换"],
    ["", "", "", "", "", "", "", ""],
    ["", "", "", "", "", "", "", ""],
    ["", "", "", "", "", "", "", ""],
    ["", "", "", "", "", "", "", ""],
], [14, 12, 16, 18, 12, 16, 16, 26])
for i, w in enumerate([14, 12, 16, 18, 12, 16, 16, 26], start=1):
    ws2.column_dimensions[get_column_letter(i)].width = w
for r in range(4, 40):
    ws2.row_dimensions[r].height = 22
# 示例行标灰
for c in range(1, 9):
    ws2.cell(row=4, column=c).font = Font(name="微软雅黑", size=10, color="8A94A6", italic=True)
dv = DataValidation(type="list", formula1='"管理员,区队长,安全科,培训员,员工"', allow_blank=True)
ws2.add_data_validation(dv)
dv.add("E4:E200")
ws2.freeze_panes = "A4"

# ---------------- Sheet3 组织架构模板 ----------------
ws3 = wb.create_sheet("组织架构")
h3 = ["部门 / 区队", "上级部门", "人数（可选）", "备注"]
title_block(ws3, "组织架构（模板）", "按矿上实际填写，含科室与区队；没有上级部门的留空", len(h3))
header_row(ws3, 3, h3)
body_rows(ws3, 4, [
    ["安全科", "", "12", "示例，请替换"],
    ["综采一队", "", "86", "示例，请替换"],
    ["综采二队", "", "78", "示例，请替换"],
    ["通风区", "", "45", "示例，请替换"],
    ["机电科", "", "36", "示例，请替换"],
], [22, 18, 14, 30])
for i, w in enumerate([22, 18, 14, 30], start=1):
    ws3.column_dimensions[get_column_letter(i)].width = w
for r in range(4, 40):
    ws3.row_dimensions[r].height = 24
for c in range(1, 5):
    ws3.cell(row=4, column=c).font = Font(name="微软雅黑", size=10, color="8A94A6", italic=True)
ws3.freeze_panes = "A4"

try:
    wb.save(OUT)
    print("saved:", OUT)
except PermissionError:
    alt = OUT.replace(".xlsx", "_精简版.xlsx")
    wb.save(alt)
    print("原文件被占用（可能正在 Excel 里打开），已另存为:", alt)
