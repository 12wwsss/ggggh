// smoke 是端到端自测程序：按验收判据跑一遍 S1–S7 的主链路。
//
//	go run ./cmd/smoke -base http://127.0.0.1:8080 -doc testdata/煤矿安全规程节选.docx
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	baseURL  = flag.String("base", "http://127.0.0.1:8080", "服务地址")
	docPath  = flag.String("doc", "testdata/煤矿安全规程节选.docx", "待入库的规程文档")
	failures int
	passed   int
	started  = time.Now()
)

func main() {
	flag.Parse()

	section("S1 健康检查")
	health := mustGet("/healthz", "")
	check(health["status"] == "ok", "healthz 返回 ok")
	checks, _ := health["checks"].(map[string]any)
	check(sub(checks, "db", "status") == "up", "db 状态 up")
	check(sub(checks, "llm", "status") == "up", "llm 状态 up")
	check(checks["tunnel"] != nil, "tunnel 状态存在")

	section("S2 登录与 RBAC")
	admin := login("admin", "Admin@123456")
	leader := login("leader01", "Leader@123456")
	worker := login("worker01", "Worker@123456")
	check(admin.token != "" && leader.token != "" && worker.token != "", "三类账号均可登录")

	code, _ := worker.call("GET", "/api/v1/users", nil)
	check(code == http.StatusForbidden, "员工访问用户管理返回 403")
	code, adminUsers := admin.call("GET", "/api/v1/users?page_size=50", nil)
	check(code == http.StatusOK, "管理员访问用户管理返回 200")
	check(len(asList(adminUsers)) >= 7, "初始化账号可见")

	// 自测专用账号：用它的口令错误来做锁定用例，避免污染真实账号。
	_, depts := admin.call("GET", "/api/v1/depts", nil)
	deptID := 2
	if l := asList(depts); len(l) > 0 {
		deptID = int(num(l[0].(map[string]any)["id"]))
	}
	code, probe := admin.call("POST", "/api/v1/users", map[string]any{
		"username": "smokeprobe", "password": "Probe@123456", "name": "自测账号",
		"role": "employee", "dept_id": deptID,
	})
	if code == http.StatusConflict {
		check(true, "自测账号已存在，复用")
	} else {
		check(code == http.StatusOK && num(probe["id"]) > 0, "创建自测账号")
	}
	probeID := int64(num(probe["id"]))
	if probeID == 0 {
		_, listed := admin.call("GET", "/api/v1/users?keyword=smokeprobe", nil)
		if l := asList(listed); len(l) > 0 {
			probeID = int64(num(l[0].(map[string]any)["id"]))
		}
	}

	// 单次输错不能锁定：先错一次，再用正确口令必须能登录。
	bad := loginCode("smokeprobe", "wrong-password")
	check(bad == http.StatusUnauthorized, "单次错误口令被拒绝")
	afterOne := loginCode("smokeprobe", "Probe@123456")
	check(afterOne == http.StatusOK, "单次输错后正确口令仍可登录（未被误锁）")

	// 连续错到阈值才锁定，管理员可解锁。
	var lockedCode int
	for i := 0; i < 6; i++ {
		lockedCode = loginCode("smokeprobe", "wrong-password")
		if lockedCode == http.StatusLocked {
			break
		}
	}
	check(lockedCode == http.StatusLocked, "连续错误达到阈值后锁定")
	code, _ = admin.call("POST", fmt.Sprintf("/api/v1/users/%d/unlock", probeID), nil)
	check(code == http.StatusOK, "锁定账号可解锁")
	check(loginCode("smokeprobe", "Probe@123456") == http.StatusOK, "解锁后可正常登录")
	section("小程序登录")
	wxCode := fmt.Sprintf("smoke-wx-%d", time.Now().UnixNano())
	_, wl := admin.call("POST", "/api/v1/auth/wx-login", map[string]any{"code": wxCode})
	check(wl["need_bind"] == true, "新 openid 返回绑定票据")
	bindTicket := str(wl["bind_ticket"])
	code, wb := admin.call("POST", "/api/v1/auth/wx-bind", map[string]any{
		"bind_ticket": bindTicket, "username": "worker01", "password": "Worker@123456",
	})
	check(code == http.StatusOK && str(wb["token"]) != "", "首次绑定工号后直接发会话")
	loginTicket := str(wb["ticket"])
	check(loginTicket != "", "绑定后返回一次性登录票据")
	code, tl := admin.call("POST", "/api/v1/auth/ticket", map[string]any{"code": loginTicket})
	check(code == http.StatusOK && str(tl["token"]) != "", "票据可换会话")
	code, _ = admin.call("POST", "/api/v1/auth/ticket", map[string]any{"code": loginTicket})
	check(code == http.StatusUnauthorized, "票据只能使用一次")
	_, wl2 := admin.call("POST", "/api/v1/auth/wx-login", map[string]any{"code": wxCode})
	check(wl2["need_bind"] != true && str(wl2["token"]) != "", "已绑定 openid 免密登录")

	_, me := worker.call("GET", "/api/v1/auth/me", nil)
	check(me["user"] != nil, "员工可读取自身信息")

	section("S3 文档上传、解析与发布")
	docID := uploadDoc(admin, *docPath, map[string]string{"category": "规程", "force": "true"}).ID
	check(docID > 0, fmt.Sprintf("上传文档成功 id=%d", docID))
	waitDoc(docID, admin, "pending_review", 90*time.Second)

	_, chunkPage := admin.call("GET", fmt.Sprintf("/api/v1/docs/%d/chunks", docID), nil)
	chunks := asList(chunkPage)
	check(len(chunks) >= 15, fmt.Sprintf("切分出 %d 个条款切片", len(chunks)))
	clauses := map[string]bool{}
	for _, c := range chunks {
		if s := str(c.(map[string]any)["clause_no"]); s != "" {
			clauses[s] = true
		}
	}
	check(len(clauses) >= 15, fmt.Sprintf("识别到 %d 个不同条款号", len(clauses)))
	check(clauses["第一百三十六条"] && clauses["第四百四十二条"], "关键条款号切分正确")

	code, _ = worker.call("POST", fmt.Sprintf("/api/v1/docs/%d/publish", docID), nil)
	check(code == http.StatusForbidden, "员工无权发布文档")

	code, _ = admin.call("POST", fmt.Sprintf("/api/v1/docs/%d/publish", docID), nil)
	check(code == http.StatusOK, "管理员发布文档成功")
	waitDoc(docID, admin, "published", 30*time.Second)

	code, _ = admin.call("POST", fmt.Sprintf("/api/v1/docs/%d/publish", docID), nil)
	check(code == http.StatusConflict, "重复发布返回 409")

	chunkID := int64(0)
	if len(chunks) > 0 {
		chunkID = int64(num(chunks[0].(map[string]any)["id"]))
	}
	// 上传去重：同一份内容再次上传应复用原文档，force 才重新入库
	section("S3 上传去重")
	dup := uploadDoc(admin, *docPath, map[string]string{"category": "规程"})
	check(dup.ID == docID && dup.Duplicate, fmt.Sprintf("相同内容重复上传复用原文档（doc=%d dup=%d duplicate=%v）", docID, dup.ID, dup.Duplicate))
	forced := uploadDoc(admin, *docPath, map[string]string{"category": "规程", "force": "true"})
	check(forced.ID != docID && !forced.Duplicate, "force=true 可强制重新入库")
	if forced.ID > 0 {
		_, _ = admin.call("DELETE", fmt.Sprintf("/api/v1/docs/%d", forced.ID), nil)
	}
	text := mustText(fmt.Sprintf("/api/v1/chunks/%d/text", chunkID), worker.token)
	check(chunkID > 0 && strings.Contains(text, "第"), "切片原文可直接打开")

	// 解析失败不能留下脏切片（方案 S3 判据）
	section("S3 解析失败不写脏数据")
	badDoc := []byte("这是一份没有条款结构的说明文档。\n第二段内容，也没有第X条。")
	code, badBody := uploadBytes("/api/v1/docs", "无条款结构.txt", admin.token, badDoc)
	badID := int64(0)
	if code == http.StatusOK {
		var env struct {
			Data struct {
				Doc struct {
					ID int64 `json:"id"`
				} `json:"doc"`
				Duplicate bool `json:"duplicate"`
			} `json:"data"`
		}
		_ = json.Unmarshal([]byte(badBody), &env)
		badID = env.Data.Doc.ID
		if env.Data.Duplicate {
			// 上一轮已上传过同样的坏文件，复用它继续验证“失败不留脏切片”
			badID = env.Data.Doc.ID
		}
	}
	check(badID > 0, "上传无条款结构文档")
	if badID > 0 {
		status := ""
		for i := 0; i < 60; i++ {
			_, d := admin.call("GET", fmt.Sprintf("/api/v1/docs/%d", badID), nil)
			status = str(d["status"])
			if status == "failed" {
				break
			}
			time.Sleep(time.Second)
		}
		check(status == "failed", "无条款结构文档解析失败")
		_, badChunks := admin.call("GET", fmt.Sprintf("/api/v1/docs/%d/chunks", badID), nil)
		check(len(asList(badChunks)) == 0, "解析失败不留切片")
		_, _ = admin.call("DELETE", fmt.Sprintf("/api/v1/docs/%d", badID), nil)
	}

	// 版本历史：发布即生成快照，可回滚，可对比
	_, verRes := admin.call("GET", fmt.Sprintf("/api/v1/docs/%d/versions", docID), nil)
	verList := asList(verRes)
	check(len(verList) >= 1, fmt.Sprintf("发布生成版本快照（%d 个）", len(verList)))
	if len(verList) > 0 {
		ver := int(num(verList[0].(map[string]any)["version"]))
		code, rolled := admin.call("POST",
			fmt.Sprintf("/api/v1/docs/%d/versions/%d/rollback", docID, ver), nil)
		check(code == http.StatusOK && int(num(rolled["version"])) > ver, "回滚生成新版本")
		newVer := int(num(rolled["version"]))
		_, diffRes := admin.call("GET",
			fmt.Sprintf("/api/v1/docs/%d/diff?from=%d&to=%d", docID, ver, newVer), nil)
		check(len(asList(map[string]any{"list": diffRes["added"]})) == 0 &&
			len(asList(map[string]any{"list": diffRes["removed"]})) == 0,
			"同版本回滚无条款差异")
	}

	section("S4 向量检索")
	var top map[string]any
	for i := 0; i < 60; i++ {
		code, res := leader.call("POST", "/api/v1/qa/search", map[string]any{
			"question": "瓦斯浓度达到1.5%时怎么办？", "top_k": 6,
		})
		if code == http.StatusOK && len(asList(res)) > 0 {
			cand := asList(res)[0].(map[string]any)
			// 嵌入还没就绪时只有词法命中，score 为 0；等到有向量分数再断言
			if num(cand["score"]) > 0 {
				top = cand
				break
			}
		}
		time.Sleep(time.Second)
	}
	check(top != nil, "向量索引已就绪")
	if top != nil {
		check(str(top["clause_no"]) == "第一百三十六条", "检索首位命中第一百三十六条")
		check(num(top["score"]) > 0, "返回相似度分数")
	}

	section("S5 问答与引用校验")
	code, ans := worker.call("POST", "/api/v1/qa/ask", map[string]any{
		"question": "瓦斯浓度达到1.5%时应当怎么处理？",
	})
	check(code == http.StatusOK && ans["answered"] == true, "可回答问题被回答")
	cites := asList(map[string]any{"list": ans["citations"]})
	check(len(cites) > 0, "回答带引用")
	if len(cites) > 0 {
		c := cites[0].(map[string]any)
		check(str(c["clause_no"]) == "第一百三十六条", "引用条款号正确")
		body := mustText(fmt.Sprintf("/api/v1/chunks/%d/text", int64(num(c["chunk_id"]))), worker.token)
		check(strings.Contains(body, "1.5%"), "引用原文可打开且内容一致")
	}
	check(strings.Contains(str(ans["answer"]), "第一百三十六条"), "答案正文包含条款号")

	acts := asList(map[string]any{"list": ans["actions"]})
	check(len(acts) >= 2, fmt.Sprintf("处置动作逐条拆分（%d 条）", len(acts)))
	if len(acts) > 0 {
		joined := ""
		for _, a := range acts {
			joined += str(a)
		}
		check(strings.Contains(joined, "切断电源"), "动作内容来自条文原文")
		check(!strings.Contains(str(ans["answer"]), "1. "), "编号动作不再留在正文里")
	}

	_, fb := worker.call("POST", "/api/v1/qa/ask", map[string]any{
		"question": "综采工作面采煤机滚筒截齿的更换周期是多少天？",
	})
	check(fb["answered"] == false, "无依据问题走兜底")
	check(strings.Contains(str(fb["answer"]), "未找到明确依据"), "兜底话术正确")

	_, logs := admin.call("GET", "/api/v1/qa/logs?page_size=50", nil)
	check(len(asList(logs)) >= 2, fmt.Sprintf("问答留痕 %d 条", len(asList(logs))))

	section("S4/S5 方案评测集")
	runRetrievalEval(admin, "testdata/eval/retrieval_20.json", 20, 0.85)
	runQAEval(admin, "testdata/eval/qa_30.json", 30, 0.9)

	section("S6 学习、小测、宣贯、考试")
	_, items := worker.call("GET", "/api/v1/learn/items", nil)
	itemList := asList(items)
	check(len(itemList) >= 4, fmt.Sprintf("学习条目 %d 条", len(itemList)))
	if len(itemList) > 0 {
		itemID := int64(num(itemList[0].(map[string]any)["id"]))
		code, rec := worker.call("POST", fmt.Sprintf("/api/v1/learn/items/%d/progress", itemID),
			map[string]any{"progress": 100, "duration_sec": 300})
		check(code == http.StatusOK && rec["status"] == "completed", "学习进度完成")

		_, qs := worker.call("GET", fmt.Sprintf("/api/v1/quiz/items/%d/questions", itemID), nil)
		qlist := asList(qs)
		check(len(qlist) >= 2, "小测题目可获取")
		if len(qlist) >= 2 {
			check(str(qlist[0].(map[string]any)["answer"]) == "", "员工端题目不含答案")
			answers := map[string]string{}
			for _, q := range qlist {
				answers[fmt.Sprint(int64(num(q.(map[string]any)["id"])))] = "B"
			}
			code, sub := worker.call("POST", fmt.Sprintf("/api/v1/quiz/items/%d/submit", itemID),
				map[string]any{"answers": answers})
			check(code == http.StatusOK && sub["score"] != nil, "小测提交并计分")
		}
	}

	_, bc := admin.call("POST", "/api/v1/broadcast/tasks", map[string]any{
		"title": "雨季顶板管理专项宣贯", "content": "各队组织学习顶板管理条款，班前会宣讲并签到。",
		"doc_ids": []int64{docID}, "require_ack": true, "publish": true,
	})
	check(int64(num(bc["id"])) > 0, "宣贯任务已发布")
	_, mine := worker.call("GET", "/api/v1/broadcast/mine?pending=true", nil)
	mineList := asList(mine)
	check(len(mineList) > 0, "员工收到待签收宣贯")
	if len(mineList) > 0 {
		taskID := int64(num(mineList[0].(map[string]any)["id"]))
		code, _ = worker.call("POST", fmt.Sprintf("/api/v1/broadcast/tasks/%d/ack", taskID),
			map[string]any{"ack": true})
		check(code == http.StatusOK, "宣贯签收成功")
	}

	_, qs := admin.call("GET", "/api/v1/quiz/items/1/questions", nil)
	paperQs := asList(qs)
	ids := make([]int64, 0, len(paperQs))
	for _, q := range paperQs {
		ids = append(ids, int64(num(q.(map[string]any)["id"])))
	}
	if len(ids) == 0 {
		ids = []int64{1, 2}
	}
	_, p := admin.call("POST", "/api/v1/exam/papers", map[string]any{
		"title": "顶板与瓦斯月度考试", "question_ids": ids, "pass_score": 60, "duration_min": 30,
	})
	paperID := int64(num(p["id"]))
	check(paperID > 0, "试卷创建成功")
	code, _ = admin.call("POST", fmt.Sprintf("/api/v1/exam/papers/%d/publish", paperID), nil)
	check(code == http.StatusOK, "试卷已发布")

	code, ex := worker.call("POST", fmt.Sprintf("/api/v1/exam/papers/%d/start", paperID), nil)
	check(code == http.StatusOK, "开始考试成功")
	examQs := asList(map[string]any{"list": ex["questions"]})
	if len(examQs) > 0 {
		check(str(examQs[0].(map[string]any)["answer"]) == "", "考试题目不含答案")
	}
	answers := map[string]string{}
	for _, q := range examQs {
		answers[fmt.Sprint(int64(num(q.(map[string]any)["id"])))] = "B"
	}
	code, sub := worker.call("POST", fmt.Sprintf("/api/v1/exam/papers/%d/submit", paperID),
		map[string]any{"answers": answers})
	check(code == http.StatusOK && sub["score"] != nil, "交卷并计分")
	code, _ = worker.call("POST", fmt.Sprintf("/api/v1/exam/papers/%d/submit", paperID),
		map[string]any{"answers": answers})
	check(code == http.StatusConflict, "重复交卷返回 409")

	section("S7 问题闭环")
	_, iss := worker.call("POST", "/api/v1/issues", map[string]any{
		"title": "1302 工作面端头支护不足", "description": "端头 3 架棚梁未按规程架设，存在空顶。",
		"severity": "high", "cate_id": 1,
	})
	issueID := int64(num(iss["id"]))
	check(issueID > 0, "问题上报成功")

	// 并发状态迁移：两个动作同时提交，事务保护下只有一个生效
	_, iss2 := worker.call("POST", "/api/v1/issues", map[string]any{
		"title": "并发受理用例", "description": "用于验证同一单据的并发受理。", "severity": "low",
	})
	concurrentID := int64(num(iss2["id"]))
	if concurrentID > 0 {
		var wg sync.WaitGroup
		codes := make([]int, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			codes[0], _ = leader.call("POST", fmt.Sprintf("/api/v1/issues/%d/accept", concurrentID),
				map[string]any{"comment": "并发 A"})
		}()
		go func() {
			defer wg.Done()
			codes[1], _ = admin.call("POST", fmt.Sprintf("/api/v1/issues/%d/accept", concurrentID),
				map[string]any{"comment": "并发 B"})
		}()
		wg.Wait()
		okCount, conflictCount := 0, 0
		for _, c := range codes {
			switch c {
			case http.StatusOK:
				okCount++
			case http.StatusConflict:
				conflictCount++
			}
		}
		check(okCount == 1 && conflictCount == 1,
			fmt.Sprintf("并发提交两个动作只有一条生效（200×%d / 409×%d）", okCount, conflictCount))
	}

	code, _ = worker.call("POST", fmt.Sprintf("/api/v1/issues/%d/rectify", issueID),
		map[string]any{"result": "未受理先整改"})
	check(code == http.StatusConflict, "跨状态操作返回 409")

	code, _ = leader.call("POST", fmt.Sprintf("/api/v1/issues/%d/accept", issueID),
		map[string]any{"comment": "立即整改"})
	check(code == http.StatusOK, "区队长受理")

	code, _ = worker.call("POST", fmt.Sprintf("/api/v1/issues/%d/rectify", issueID),
		map[string]any{"result": "已补架棚梁并加固，现场验收合格"})
	check(code == http.StatusOK, "整改提交")

	code, _ = leader.call("POST", fmt.Sprintf("/api/v1/issues/%d/verify", issueID),
		map[string]any{"pass": true, "comment": "现场验收通过"})
	check(code == http.StatusOK, "验收通过")

	code, _ = leader.call("POST", fmt.Sprintf("/api/v1/issues/%d/close", issueID),
		map[string]any{"comment": "闭环"})
	check(code == http.StatusOK, "问题闭环")

	_, flow := admin.call("GET", fmt.Sprintf("/api/v1/issues/%d/flow", issueID), nil)
	check(len(asList(flow)) == 5, fmt.Sprintf("流转记录 %d 条且只增不改", len(asList(flow))))

	_, stats := leader.call("GET", "/api/v1/issues/stats", nil)
	check(stats["closed"] != nil || stats["verified"] != nil, "问题统计可读")

	// 附件：上传后必须能按原内容下载回来
	payload := []byte("隐患现场照片占位内容 2026-11-12")
	code, attBody := uploadBytes(fmt.Sprintf("/api/v1/issues/%d/attachments", issueID),
		"现场照片.txt", worker.token, payload)
	check(code == http.StatusOK, "附件上传成功")
	attID := int64(0)
	if code == http.StatusOK {
		var env struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		_ = json.Unmarshal([]byte(attBody), &env)
		attID = env.Data.ID
	}
	check(attID > 0, "附件返回记录 id")
	if attID > 0 {
		code, got := getBytes(fmt.Sprintf("/api/v1/issues/%d/attachments/%d", issueID, attID), worker.token)
		check(code == http.StatusOK && bytes.Equal(got, payload), "附件可下载且内容一致")
	}

	section("看板与导出")
	_, dash := admin.call("GET", "/api/v1/dashboard", nil)
	check(dash["learn_completion"] != nil, "学习完成率可读")
	check(dash["exam_pass_rate"] != nil, "考试通过率可读")
	check(dash["broadcast_ack_rate"] != nil, "宣贯签收率可读")
	check(fmt.Sprint(dash["chunks"]) != "0", "切片计数非零")

	code, body := admin.raw("GET", "/api/v1/learn/export")
	check(code == http.StatusOK && strings.Contains(body, "姓名"), "学习记录可导出 CSV")
	code, body = admin.raw("GET", "/api/v1/exam/export")
	check(code == http.StatusOK && strings.Contains(body, "部门"), "考试成绩可导出 CSV")
	code, body = admin.raw("GET", "/api/v1/audit/export")
	check(code == http.StatusOK && strings.Contains(body, "时间"), "审计日志可导出 CSV")

	metricsCode, metricsBody := admin.raw("GET", "/metrics")
	check(metricsCode == http.StatusOK, "metrics 接口可访问")
	check(strings.Contains(metricsBody, "coalrag_docs_published") &&
		strings.Contains(metricsBody, "coalrag_http_requests_total"), "metrics 输出关键指标")

	_, tasks := admin.call("GET", "/api/v1/tasks?page_size=50", nil)
	taskList := asList(tasks)
	check(len(taskList) > 0, fmt.Sprintf("任务表记录 %d 条", len(taskList)))
	// 解析失败用例会合法地产生一条 failed 的 doc_parse 任务，其余类型不应失败。
	unexpectedFailed := 0
	failedParse := 0
	for _, t := range taskList {
		m := t.(map[string]any)
		if str(m["status"]) != "failed" {
			continue
		}
		if str(m["kind"]) == "doc_parse" {
			failedParse++
			continue
		}
		unexpectedFailed++
	}
	check(unexpectedFailed == 0, fmt.Sprintf("无异常失败任务（解析失败 %d 条为用例预期）", failedParse))

	_, audit := admin.call("GET", "/api/v1/audit?page_size=20", nil)
	check(len(asList(audit)) > 0, "审计日志有记录")

	fmt.Printf("\n===== 自测结果：通过 %d 项，失败 %d 项，用时 %s =====\n",
		passed, failures, time.Since(started).Round(time.Second))
	if failures > 0 {
		os.Exit(1)
	}
	fmt.Println("全部通过")
}

// ---------- 工具 ----------

type evalCase struct {
	Question     string `json:"question"`
	ExpectClause string `json:"expect_clause"`
	Answerable   bool   `json:"answerable"`
}

func loadEvalCases(path string) ([]evalCase, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f struct {
		Cases []evalCase `json:"cases"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	return f.Cases, nil
}

// runRetrievalEval 用方案要求的题量跑检索评测：命中率可统计并留档。
func runRetrievalEval(s *session, path string, wantTotal int, minRate float64) {
	cases, err := loadEvalCases(path)
	if err != nil {
		check(false, "读取评测集 "+path+": "+err.Error())
		return
	}
	check(len(cases) == wantTotal, fmt.Sprintf("检索评测集 %d 题（方案要求 %d）", len(cases), wantTotal))
	code, rr := s.call("POST", "/api/v1/eval/retrieval", map[string]any{"cases": cases, "top_k": 6})
	report, _ := rr["report"].(map[string]any)
	check(code == http.StatusOK && int(num(report["total"])) == wantTotal, "检索评测覆盖全部题目")
	check(num(report["hit_rate"]) >= minRate,
		fmt.Sprintf("检索 topK=6 命中率 %.2f（%d/%d）", num(report["hit_rate"]), int(num(report["hits"])), int(num(report["total"]))))
	check(rr["saved"] != nil, "检索评测报告已落盘")
}

// runQAEval 跑完整问答链路：可回答的必须带正确条款，不可回答的必须兜底。
func runQAEval(s *session, path string, wantTotal int, minAcc float64) {
	cases, err := loadEvalCases(path)
	if err != nil {
		check(false, "读取评测集 "+path+": "+err.Error())
		return
	}
	check(len(cases) == wantTotal, fmt.Sprintf("问答评测集 %d 题（方案要求 %d）", len(cases), wantTotal))
	code, qr := s.call("POST", "/api/v1/eval/qa", map[string]any{"cases": cases})
	report, _ := qr["report"].(map[string]any)
	check(code == http.StatusOK && int(num(report["total"])) == wantTotal, "问答评测覆盖全部题目")
	check(num(report["accuracy"]) >= minAcc, fmt.Sprintf("问答评测准确率 %.2f", num(report["accuracy"])))

	// 方案 S5 的判据：可回答的必须带条款号且原文可打开（引用只能来自检索结果，
	// 服务端已校验），无依据的走兜底、不得编造条款号。逐条核对这两点。
	missingCitation := 0
	items, _ := report["items"].([]any)
	for _, raw := range items {
		it, _ := raw.(map[string]any)
		cites, _ := it["citations"].([]any)
		if it["answered"] == true && len(cites) == 0 {
			missingCitation++
		}
	}
	check(missingCitation == 0, "已回答的问题都带条款号")
	check(num(report["mismatch"]) <= 1,
		fmt.Sprintf("错引条款 %d 条（相邻条款混淆，靠评测暴露）", int(num(report["mismatch"]))))
	check(num(report["false_positive"]) <= 1,
		fmt.Sprintf("不可回答问题误答 %d 条", int(num(report["false_positive"]))))
	check(num(report["answered"])+num(report["fallback"]) == float64(wantTotal), "回答与兜底数量守恒")
	check(qr["saved"] != nil, "问答评测报告已落盘")
}

func section(name string) { fmt.Printf("\n----- %s -----\n", name) }

func check(cond bool, msg string) {
	if cond {
		passed++
		fmt.Printf("  PASS  %s\n", msg)
		return
	}
	failures++
	fmt.Printf("  FAIL  %s\n", msg)
}

type session struct{ token string }

func login(username, password string) *session {
	code, body := callRaw("POST", "/api/v1/auth/login", "", map[string]any{
		"username": username, "password": password,
	})
	if code != http.StatusOK {
		fmt.Printf("  ERR   登录 %s 失败: %d %v\n", username, code, body["message"])
		return &session{}
	}
	data, _ := body["data"].(map[string]any)
	token, _ := data["token"].(string)
	return &session{token: token}
}

func loginCode(username, password string) int {
	code, _ := callRaw("POST", "/api/v1/auth/login", "", map[string]any{
		"username": username, "password": password,
	})
	return code
}

// call 统一把响应 data 归一为 map；数组数据放进 list 字段。
func (s *session) call(method, path string, payload any) (int, map[string]any) {
	code, body := callRaw(method, path, s.token, payload)
	out := map[string]any{}
	switch d := body["data"].(type) {
	case map[string]any:
		out = d
	case []any:
		out["list"] = d
	}
	if msg, ok := body["message"].(string); ok {
		out["_message"] = msg
	}
	return code, out
}

func (s *session) raw(method, path string) (int, string) {
	return rawRequest(method, path, s.token, nil)
}

func mustGet(path, token string) map[string]any {
	_, body := callRaw("GET", path, token, nil)
	return body
}

func mustText(path, token string) string {
	code, body := rawRequest("GET", path, token, nil)
	if code != http.StatusOK {
		return ""
	}
	return body
}

func callRaw(method, path, token string, payload any) (int, map[string]any) {
	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
	}
	code, text := rawRequest(method, path, token, body)
	out := map[string]any{}
	if text != "" {
		_ = json.Unmarshal([]byte(text), &out)
	}
	return code, out
}

func rawRequest(method, path, token string, body io.Reader) (int, string) {
	req, err := http.NewRequest(method, *baseURL+path, body)
	if err != nil {
		fmt.Printf("  ERR   构造请求失败: %v\n", err)
		return 0, ""
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("  ERR   请求 %s %s 失败: %v\n", method, path, err)
		return 0, ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// uploadBytes 以 multipart 上传内存内容，返回状态码与响应体。
func uploadBytes(path, filename, token string, content []byte) (int, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return 0, ""
	}
	if _, err := part.Write(content); err != nil {
		return 0, ""
	}
	if err := w.Close(); err != nil {
		return 0, ""
	}
	req, err := http.NewRequest("POST", *baseURL+path, &buf)
	if err != nil {
		return 0, ""
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// getBytes 下载二进制内容。
func getBytes(path, token string) (int, []byte) {
	req, err := http.NewRequest("GET", *baseURL+path, nil)
	if err != nil {
		return 0, nil
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

type uploadResult struct {
	ID        int64
	Duplicate bool
}

// uploadDoc 上传文档并解出 {doc, duplicate} 结构。
func uploadDoc(s *session, path string, fields map[string]string) uploadResult {
	f, err := os.Open(path)
	if err != nil {
		fmt.Printf("  ERR   打开文档失败: %v\n", err)
		return uploadResult{}
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return uploadResult{}
	}
	if _, err := io.Copy(part, f); err != nil {
		return uploadResult{}
	}
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	_ = w.Close()

	req, _ := http.NewRequest("POST", *baseURL+"/api/v1/docs", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+s.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("  ERR   上传失败: %v\n", err)
		return uploadResult{}
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var out struct {
		Data struct {
			Doc struct {
				ID int64 `json:"id"`
			} `json:"doc"`
			Duplicate bool `json:"duplicate"`
		} `json:"data"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(b, &out)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("  ERR   上传返回 %d: %s\n", resp.StatusCode, out.Message)
	}
	return uploadResult{ID: out.Data.Doc.ID, Duplicate: out.Data.Duplicate}
}

func waitDoc(docID int64, s *session, want string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	last := ""
	for time.Now().Before(deadline) {
		_, doc := s.call("GET", fmt.Sprintf("/api/v1/docs/%d", docID), nil)
		last = str(doc["status"])
		if last == want {
			check(true, fmt.Sprintf("文档状态进入 %s", want))
			return
		}
		if last == "failed" {
			check(false, "文档解析失败: "+str(doc["error_msg"]))
			return
		}
		time.Sleep(time.Second)
	}
	check(false, fmt.Sprintf("等待文档状态 %s 超时，当前 %s", want, last))
}

func asList(m map[string]any) []any {
	if v, ok := m["list"].([]any); ok {
		return v
	}
	if v, ok := m["hits"].([]any); ok {
		return v
	}
	return nil
}

func sub(m map[string]any, key, field string) string {
	if v, ok := m[key].(map[string]any); ok {
		return str(v[field])
	}
	return ""
}

func str(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func num(v any) float64 {
	f, _ := v.(float64)
	return f
}
