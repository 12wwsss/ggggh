// Package metrics 提供轻量指标登记与 Prometheus 文本输出，
// 不引入额外依赖，够内网抓取与排障使用。
package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Registry 保存计数器与瞬时值。
type Registry struct {
	mu      sync.RWMutex
	values  map[string]float64
	kinds   map[string]string
	started time.Time
}

// New 构造登记表。
func New() *Registry {
	return &Registry{values: map[string]float64{}, kinds: map[string]string{}, started: time.Now()}
}

// Inc 计数加一。
func (r *Registry) Inc(name string, labels map[string]string) {
	r.Add(name, labels, 1)
}

// Add 计数累加。
func (r *Registry) Add(name string, labels map[string]string, v float64) {
	key := series(name, labels)
	r.mu.Lock()
	if _, ok := r.kinds[name]; !ok {
		r.kinds[name] = "counter"
	}
	r.values[key] += v
	r.mu.Unlock()
}

// Set 设置瞬时值。
func (r *Registry) Set(name string, labels map[string]string, v float64) {
	key := series(name, labels)
	r.mu.Lock()
	r.kinds[name] = "gauge"
	r.values[key] = v
	r.mu.Unlock()
}

// Render 输出 Prometheus 文本格式。
func (r *Registry) Render() string {
	r.mu.RLock()
	keys := make([]string, 0, len(r.values))
	for k := range r.values {
		keys = append(keys, k)
	}
	vals := make(map[string]float64, len(r.values))
	for k, v := range r.values {
		vals[k] = v
	}
	kinds := make(map[string]string, len(r.kinds))
	for k, v := range r.kinds {
		kinds[k] = v
	}
	r.mu.RUnlock()
	sort.Strings(keys)

	var b strings.Builder
	fmt.Fprintf(&b, "# HELP coalrag_uptime_seconds 进程运行时长\n# TYPE coalrag_uptime_seconds gauge\ncoalrag_uptime_seconds %.0f\n",
		time.Since(r.started).Seconds())

	lastName := ""
	for _, k := range keys {
		name := k
		if i := strings.IndexByte(k, '{'); i > 0 {
			name = k[:i]
		}
		if name != lastName {
			kind := kinds[name]
			if kind == "" {
				kind = "gauge"
			}
			fmt.Fprintf(&b, "# TYPE coalrag_%s %s\n", name, kind)
			lastName = name
		}
		fmt.Fprintf(&b, "coalrag_%s %g\n", k, vals[k])
	}
	return b.String()
}

func series(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, escape(labels[k])))
	}
	return name + "{" + strings.Join(parts, ",") + "}"
}

func escape(v string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(v)
}
