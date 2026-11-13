// envprobe 采集部署环境信息，输出 env-report.txt（对应方案 S0）。
//
//	go run ./cmd/envprobe -out env-report.txt -dsn "postgres://..."
//
// 每项给出结论；采集不到的项明确标注“未知”，不做猜测。
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"coalrag/internal/config"
	"coalrag/internal/db"
	"coalrag/internal/llm"
)

var lines []string

func main() {
	out := flag.String("out", "env-report.txt", "输出文件")
	dsn := flag.String("dsn", config.Default().DSN, "数据库连接串")
	llmBase := flag.String("llm-base", config.Default().LLM.BaseURL, "模型服务地址")
	llmKey := flag.String("llm-key", "", "模型服务密钥")
	embedModel := flag.String("embed-model", config.Default().Embed.Model, "嵌入模型名")
	flag.Parse()

	section("一、主机与操作系统")
	host, _ := os.Hostname()
	line("主机名", host, "OK")
	line("操作系统", fmt.Sprintf("%s %s", runtime.GOOS, osRelease()), "OK")
	line("CPU 架构", runtime.GOARCH, "OK")
	line("CPU 逻辑核数", fmt.Sprint(runtime.NumCPU()), "OK")
	line("glibc 版本", glibcVersion(), "")
	line("Go 运行时", runtime.Version(), "OK")

	section("二、内存与磁盘")
	total, free, memOk := memoryInfo()
	line("物理内存", total, statusOr(memOk))
	line("可用内存", free, statusOr(memOk))
	freeDisk, diskOk := diskFree()
	line("当前磁盘可用空间", freeDisk, statusOr(diskOk))

	section("三、GPU 与驱动")
	gpu, gpuOK := gpuInfo()
	line("GPU 型号与显存", gpu, statusOr(gpuOK))
	if !gpuOK {
		line("结论", "未检测到 nvidia-smi；若目标机为昇腾请改用 npu-smi 核对", "未知")
	}

	section("四、数据库")
	probeDatabase(dsn)

	section("五、模型接口")
	probeLLM(*llmBase, *llmKey, *embedModel)

	section("六、出站能力")
	probeOutbound()

	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(*out, []byte(body), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "写入报告失败:", err)
		os.Exit(1)
	}
	fmt.Print(body)
	fmt.Printf("\n报告已写入 %s\n", *out)
}

func section(title string) {
	lines = append(lines, "", "== "+title+" ==")
	fmt.Println("\n== " + title + " ==")
}

// line 输出一项结论；status 为空时按 detail 是否以“未知”开头推断。
func line(name, detail, status string) {
	if status == "" {
		status = "OK"
		if detail == "" || strings.HasPrefix(detail, "未知") {
			status = "未知"
			if detail == "" {
				detail = "未知"
			}
		}
	}
	text := fmt.Sprintf("%-16s %-60s [%s]", name, detail, status)
	lines = append(lines, text)
	fmt.Println(text)
}

func statusOr(ok bool) string {
	if ok {
		return "OK"
	}
	return "未知"
}

func run(name string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func osRelease() string {
	if runtime.GOOS == "windows" {
		if s, ok := run("cmd", "/c", "ver"); ok {
			return s
		}
		return "未知"
	}
	if b, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, l := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(l, "PRETTY_NAME=") {
				return strings.Trim(strings.TrimPrefix(l, "PRETTY_NAME="), `"`)
			}
		}
	}
	return "未知"
}

func glibcVersion() string {
	if runtime.GOOS != "linux" {
		return "不适用（非 Linux）"
	}
	if s, ok := run("ldd", "--version"); ok {
		if i := strings.IndexByte(s, '\n'); i > 0 {
			return strings.TrimSpace(s[:i])
		}
		return strings.TrimSpace(s)
	}
	return "未知"
}

func memoryInfo() (string, string, bool) {
	if runtime.GOOS == "windows" {
		s, ok := run("wmic", "OS", "get", "TotalVisibleMemorySize,FreePhysicalMemory", "/value")
		if !ok {
			return "", "", false
		}
		total, free := "", ""
		for _, l := range strings.Split(s, "\r\n") {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "TotalVisibleMemorySize=") {
				total = strings.TrimPrefix(l, "TotalVisibleMemorySize=") + " KB"
			}
			if strings.HasPrefix(l, "FreePhysicalMemory=") {
				free = strings.TrimPrefix(l, "FreePhysicalMemory=") + " KB"
			}
		}
		if total == "" {
			return "", "", false
		}
		return total, free, true
	}
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return "", "", false
	}
	var total, avail string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "MemTotal:") {
			total = strings.TrimSpace(strings.TrimPrefix(l, "MemTotal:"))
		}
		if strings.HasPrefix(l, "MemAvailable:") {
			avail = strings.TrimSpace(strings.TrimPrefix(l, "MemAvailable:"))
		}
	}
	if total == "" {
		return "", "", false
	}
	return total, avail, true
}

func diskFree() (string, bool) {
	if runtime.GOOS == "windows" {
		s, ok := run("wmic", "logicaldisk", "where", "DeviceID='C:'", "get", "FreeSpace,Size", "/value")
		if !ok {
			return "", false
		}
		free := ""
		for _, l := range strings.Split(s, "\r\n") {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "FreeSpace=") {
				var n int64
				if _, err := fmt.Sscan(strings.TrimPrefix(l, "FreeSpace="), &n); err == nil {
					free = fmt.Sprintf("%.1f GB", float64(n)/1e9)
				}
			}
		}
		if free == "" {
			return "", false
		}
		return free, true
	}
	s, ok := run("df", "-BG", "/")
	if !ok {
		return "", false
	}
	parts := strings.Fields(s)
	if len(parts) < 4 {
		return "", false
	}
	// 取最后一个可用列（df 输出最后一行为目标挂载点）
	rows := strings.Split(s, "\n")
	last := strings.Fields(rows[len(rows)-1])
	if len(last) >= 4 {
		return last[3], true
	}
	return parts[3], true
}

func gpuInfo() (string, bool) {
	s, ok := run("nvidia-smi", "--query-gpu=name,memory.total,driver_version", "--format=csv,noheader")
	if !ok {
		return "未检测到 nvidia-smi", false
	}
	return strings.ReplaceAll(s, "\n", "; "), true
}

func probeDatabase(dsn *string) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, *dsn)
	if err != nil {
		line("连接", err.Error(), "未知")
		line("结论", "数据库不可达，无法核对版本与扩展", "未知")
		return
	}
	defer pool.Close()

	var version string
	if err := pool.QueryRow(ctx, "select version()").Scan(&version); err != nil {
		line("版本", err.Error(), "未知")
	} else {
		line("版本", version, "OK")
	}

	rows, err := pool.Query(ctx, `select name, default_version, coalesce(comment,'')
		from pg_available_extensions where name in ('vector','pg_trgm','uuid-ossp') order by name`)
	if err != nil {
		line("可用扩展", err.Error(), "未知")
		return
	}
	defer rows.Close()
	found := 0
	for rows.Next() {
		var name, ver, comment string
		if err := rows.Scan(&name, &ver, &comment); err != nil {
			continue
		}
		found++
		status := "OK"
		if name == "vector" {
			var installed bool
			_ = pool.QueryRow(ctx, `select exists(select 1 from pg_extension where extname='vector')`).Scan(&installed)
			if installed {
				status = "已安装"
			} else {
				status = "可用未安装"
			}
		}
		line("扩展 "+name, ver, status)
	}
	if found == 0 {
		line("可用扩展", "未发现 vector/pg_trgm/uuid-ossp", "未知")
	}
}

func probeLLM(base, key, embedModel string) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	chatCfg := config.Default().LLM
	chatCfg.BaseURL = base
	chatCfg.APIKey = key
	chat := llm.NewOpenAIChat(chatCfg)
	if err := chat.Ping(ctx); err != nil {
		line("对话接口", fmt.Sprintf("%s 不可达: %v", base, err), "未知")
	} else {
		line("对话接口", base+" 可达", "OK")
	}

	embCfg := config.Default().Embed
	embCfg.BaseURL = base
	embCfg.APIKey = key
	emb := llm.NewOpenAIEmbedder(embCfg)
	if err := emb.Ping(ctx); err != nil {
		line("嵌入接口", fmt.Sprintf("%s 不可达: %v", base, err), "未知")
		return
	}
	vecs, err := emb.Embed(ctx, []string{"煤矿安全规程"})
	if err != nil {
		line("嵌入调用", fmt.Sprintf("%s 调用失败: %v", embedModel, err), "未知")
		return
	}
	line("嵌入维度", fmt.Sprintf("%d（模型 %s）", len(vecs[0]), embedModel), "OK")
}

func probeOutbound() {
	targets := []string{"223.5.5.5:443", "www.baidu.com:443"}
	for _, t := range targets {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", t, 3*time.Second)
		if err != nil {
			line("出站 "+t, err.Error(), "未知")
			continue
		}
		_ = conn.Close()
		line("出站 "+t, fmt.Sprintf("可达，耗时 %dms", time.Since(start).Milliseconds()), "OK")
	}
}
