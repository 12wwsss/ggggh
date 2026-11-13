// Package bootstrap 负责空库初始化：部门、账号、示例学习条目与小测。
package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"coalrag/internal/auth"
	"coalrag/internal/model"
	"coalrag/internal/store"
)

// Run 在用户表为空时写入演示数据，返回是否执行了初始化。
func Run(ctx context.Context, st *store.Store) (bool, error) {
	users, _, err := st.ListUsers(ctx, nil, "", 1, 0)
	if err != nil {
		return false, err
	}
	if len(users) > 0 {
		return false, nil
	}
	depts := []struct {
		name, code string
	}{
		{"矿部", "MINE"},
		{"综采一队", "DU1"},
		{"综采二队", "DU2"},
		{"通风区", "VENT"},
		{"机电科", "ELEC"},
	}
	deptIDs := map[string]int64{}
	for i, d := range depts {
		id, err := st.CreateDept(ctx, &model.Dept{Name: d.name, Code: d.code, Sort: (i + 1) * 10})
		if err != nil {
			return false, fmt.Errorf("初始化部门 %s: %w", d.name, err)
		}
		deptIDs[d.code] = id
	}

	seedUsers := []struct {
		username, password, name, role, dept, post string
	}{
		{"admin", "Admin@123456", "系统管理员", model.RoleAdmin, "MINE", "安全副矿长"},
		{"leader01", "Leader@123456", "李建国", model.RoleDeptLeader, "DU1", "综采一队队长"},
		{"leader02", "Leader@123456", "王海涛", model.RoleDeptLeader, "DU2", "综采二队队长"},
		{"worker01", "Worker@123456", "张明", model.RoleEmployee, "DU1", "采煤机司机"},
		{"worker02", "Worker@123456", "刘洋", model.RoleEmployee, "DU1", "支架工"},
		{"worker03", "Worker@123456", "赵磊", model.RoleEmployee, "DU2", "电工"},
		{"worker04", "Worker@123456", "陈刚", model.RoleEmployee, "VENT", "瓦斯检查工"},
	}
	for _, u := range seedUsers {
		hash, err := auth.HashPassword(u.password)
		if err != nil {
			return false, err
		}
		deptID := deptIDs[u.dept]
		if _, err := st.CreateUser(ctx, &model.User{
			Username: u.username, Name: u.name, Role: u.role, Status: "active",
			DeptID: &deptID, Post: u.post,
		}, hash); err != nil {
			return false, fmt.Errorf("初始化账号 %s: %w", u.username, err)
		}
	}

	items := []model.LearnItem{
		{
			Title: "顶板管理：敲帮问顶与临时支护",
			Post:  "支架工", Category: "顶板管理", Sort: 10, Status: "published",
			Content: "进入作业地点前必须先敲帮问顶，由班组长或有经验的人员站在支护完好的地点，" +
				"用长柄工具由外向内、自上而下逐个检查。发现活矸危岩必须立即处理，处理时下方不得有人。" +
				"临时支护未到位不得进入煤壁侧作业。",
			ClauseRefs: []string{"第五十五条", "第五十六条"},
		},
		{
			Title: "瓦斯检查：巡回检查与三对口",
			Post:  "瓦斯检查工", Category: "通风瓦斯", Sort: 20, Status: "published",
			Content: "瓦斯检查工必须按巡回检查计划逐点检查，每次检查结果记入瓦斯检查手册和检查地点记录牌，" +
				"并做到手册、牌板、班报三对口。发现瓦斯超限立即停止作业、撤出人员并报告。",
			ClauseRefs: []string{"第一百三十五条"},
		},
		{
			Title: "机电运输：停送电与挂牌闭锁",
			Post:  "电工", Category: "机电运输", Sort: 30, Status: "published",
			Content: "检修电气设备必须执行停电、验电、放电、挂牌、闭锁、专人监护制度。" +
				"严禁带电作业，严禁约时停送电。恢复送电前必须确认人员撤至安全地点。",
			ClauseRefs: []string{"第四百四十二条"},
		},
		{
			Title: "岗位安全责任：班前会与交接班",
			Post:  "", Category: "安全责任", Sort: 40, Status: "published",
			Content: "班前会必须交代当班安全风险与防范措施，检查人员精神状态。" +
				"交接班必须交清现场隐患、设备状况和未完成的整改事项。",
			ClauseRefs: []string{},
		},
	}

	for i := range items {
		id, err := st.CreateLearnItem(ctx, &items[i])
		if err != nil {
			return false, fmt.Errorf("初始化学习条目: %w", err)
		}
		items[i].ID = id
	}

	quizzes := []model.QuizQuestion{
		{ItemID: &items[0].ID, Stem: "进入作业地点前首先应当做什么？",
			Options: []string{"A. 直接开始作业", "B. 敲帮问顶", "C. 先开运输机", "D. 等待班长"},
			Answer:  "B", Analysis: "顶板管理要求进入作业地点前先敲帮问顶。", Difficulty: "easy"},
		{ItemID: &items[0].ID, Stem: "临时支护未到位时能否进入煤壁侧作业？",
			Options: []string{"A. 可以，注意观察", "B. 不可以", "C. 经班长同意可以", "D. 有经验可以"},
			Answer:  "B", Analysis: "临时支护未到位不得进入煤壁侧作业。", Difficulty: "normal"},
		{ItemID: &items[1].ID, Stem: "瓦斯检查“三对口”指手册、牌板与什么一致？",
			Options: []string{"A. 班报", "B. 交接班记录", "C. 安全规程", "D. 调度记录"},
			Answer:  "A", Analysis: "手册、牌板、班报三对口。", Difficulty: "normal"},
		{ItemID: &items[1].ID, Stem: "发现瓦斯超限时正确做法是？",
			Options: []string{"A. 继续作业并汇报", "B. 停止作业、撤人并报告", "C. 加大风量后继续", "D. 等待下一班处理"},
			Answer:  "B", Analysis: "立即停止作业、撤出人员并报告。", Difficulty: "easy"},
		{ItemID: &items[2].ID, Stem: "检修电气设备的正确顺序是？",
			Options: []string{"A. 停电、验电、放电、挂牌、闭锁、专人监护", "B. 挂牌后直接检修", "C. 带电作业", "D. 约时送电"},
			Answer:  "A", Analysis: "必须执行停电、验电、放电、挂牌、闭锁、专人监护。", Difficulty: "normal"},
		{ItemID: &items[3].ID, Stem: "交接班必须交清的内容包括？",
			Options: []string{"A. 只看设备", "B. 现场隐患、设备状况、未完成整改事项", "C. 只需签字", "D. 口头说一声即可"},
			Answer:  "B", Analysis: "交接班必须交清现场隐患、设备状况和未完成的整改事项。", Difficulty: "easy"},
	}
	for i := range quizzes {
		if _, err := st.CreateQuizQuestion(ctx, &quizzes[i]); err != nil {
			return false, fmt.Errorf("初始化小测题: %w", err)
		}
	}
	return true, nil
}

// CheckEmbedDim 校验配置维度与库内记录一致；首次启动写入基线。
func CheckEmbedDim(ctx context.Context, st *store.Store, dim int, modelName string) error {
	key := "embed_dim"
	v, err := st.GetMeta(ctx, key)
	if errors.Is(err, store.ErrNotFound) {
		if err := st.PutMeta(ctx, key, fmt.Sprintf("%d", dim)); err != nil {
			return err
		}
		return st.PutMeta(ctx, "embed_model", modelName)
	}
	if err != nil {
		return err
	}
	var stored int
	if _, err := fmt.Sscanf(v, "%d", &stored); err != nil {
		return fmt.Errorf("元数据 %s 值 %q 无法解析", key, v)
	}
	if stored != dim {
		return fmt.Errorf("向量维度不一致：库内为 %d，配置为 %d。请修正配置或重建向量索引", stored, dim)
	}
	// 元数据只是基线，已入库的向量行也要一致，否则检索会静默算错。
	bad, err := st.CountEmbeddingsWithDim(ctx, dim)
	if err != nil {
		return err
	}
	if bad > 0 {
		return fmt.Errorf("向量表中有 %d 条记录维度不是 %d，与配置不一致，拒绝启动；请重建向量", bad, dim)
	}
	return nil
}
