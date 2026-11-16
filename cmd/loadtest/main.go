// loadtest 是内置压测工具：登录后按比例打问答/检索接口，输出 QPS 与延迟分位。
//
//	go run ./cmd/loadtest -base http://127.0.0.1:8080 -users 20 -duration 20s
//	go run ./cmd/loadtest -users 50 -duration 60s -mix ask,search -out load.json
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type result struct {
	latency time.Duration
	status  int
	err     string
}

func main() {
	base := flag.String("base", "http://127.0.0.1:8080", "服务地址")
	users := flag.Int("users", 20, "并发用户数")
	duration := flag.Duration("duration", 20*time.Second, "压测时长")
	username := flag.String("user", "worker01", "压测账号")
	password := flag.String("pass", "Worker@123456", "压测口令")
	mix := flag.String("mix", "ask,search", "请求类型及权重，如 ask,search 或 ask,ask,search")
	questions := flag.String("questions", "", "问题集（评测集 JSON 或每行一个问题），用于模拟真实用户的不同提问")
	out := flag.String("out", "", "把结果写入 JSON 文件")
	flag.Parse()

	token, err := login(*base, *username, *password)
	if err != nil {
		fmt.Println("登录失败:", err)
		os.Exit(1)
	}
	prompts := loadQuestions(*questions)
	if len(prompts) == 0 {
		prompts = []string{
			"瓦斯浓度达到1.5%时应当怎么处理？",
			"停风时怎么处理？",
			"敲帮问顶应该怎么做？",
		}
	}

	kinds := strings.Split(*mix, ",")
	for i := range kinds {
		kinds[i] = strings.TrimSpace(kinds[i])
	}

	deadline := time.Now().Add(*duration)
	results := make(chan result, 4096)
	var sent int64
	var wg sync.WaitGroup

	start := time.Now()
	for i := 0; i < *users; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			client := &http.Client{Timeout: 120 * time.Second}
			seq := 0
			for time.Now().Before(deadline) {
				kind := kinds[(worker+seq)%len(kinds)]
				seq++
				r := doOne(client, *base, token, kind, prompts[(worker*7+seq)%len(prompts)])
				atomic.AddInt64(&sent, 1)
				select {
				case results <- r:
				default:
				}
			}
		}(i)
	}
	go func() { wg.Wait(); close(results) }()

	var (
		latencies []time.Duration
		codes     = map[int]int{}
		errs      = map[string]int{}
	)
	for r := range results {
		latencies = append(latencies, r.latency)
		codes[r.status]++
		if r.err != "" {
			errs[r.err]++
		}
	}
	elapsed := time.Since(start)

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	report := map[string]any{
		"users":         *users,
		"duration_s":    math.Round(elapsed.Seconds()*10) / 10,
		"requests":      len(latencies),
		"qps":           math.Round(float64(len(latencies))/elapsed.Seconds()*100) / 100,
		"p50_ms":        ms(percentile(latencies, 0.50)),
		"p95_ms":        ms(percentile(latencies, 0.95)),
		"p99_ms":        ms(percentile(latencies, 0.99)),
		"max_ms":        ms(latencies[len(latencies)-1]),
		"status":        codes,
		"errors":        errs,
		"requests_sent": atomic.LoadInt64(&sent),
	}
	body, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(body))
	if *out != "" {
		_ = os.WriteFile(*out, body, 0o644)
	}
}

func login(base, user, pass string) (string, error) {
	payload, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	resp, err := http.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Data.Token == "" {
		return "", fmt.Errorf("返回 %d: %s", resp.StatusCode, out.Message)
	}
	return out.Data.Token, nil
}

func doOne(client *http.Client, base, token, kind, question string) result {
	var path string
	var payload any
	switch kind {
	case "search":
		path = "/api/v1/qa/search"
		payload = map[string]any{"question": question, "top_k": 6}
	default:
		path = "/api/v1/qa/ask"
		payload = map[string]any{"question": question}
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return result{latency: time.Since(start), err: "network"}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return result{latency: time.Since(start), status: resp.StatusCode}
}

// loadQuestions 从评测集 JSON（{"cases":[{"question":...}]}）或纯文本（每行一个问题）读入问题。
// 同一句话反复压测会命中 llama.cpp 的前缀缓存，QPS 会被显著高估，所以默认要求轮换问题。
func loadQuestions(path string) []string {
	if path == "" {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("读取问题集失败:", err)
		os.Exit(1)
	}
	var f struct {
		Cases []struct {
			Question string `json:"question"`
		} `json:"cases"`
	}
	if json.Unmarshal(b, &f) == nil && len(f.Cases) > 0 {
		out := make([]string, 0, len(f.Cases))
		for _, c := range f.Cases {
			if strings.TrimSpace(c.Question) != "" {
				out = append(out, c.Question)
			}
		}
		return out
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func ms(d time.Duration) float64 {
	return math.Round(float64(d.Microseconds())/1000*10) / 10
}
