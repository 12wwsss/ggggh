<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api, download } from '../../shared/http'

const tab = ref('learning')
const loading = ref(false)
const learning = ref({ rows: [], users: 0, items: 0, completed: 0 })
const wrong = ref({ rows: [], records: 0, users: 0 })
const issues = ref({ daily: [], status: {}, overdue: 0, total: 0, closed: 0, avg_close_hours: 0, by_dept: [] })
const issueDays = ref(14)

async function loadIssues() {
  try {
    issues.value = await api.get('/api/v1/reports/issues', { days: issueDays.value })
  } catch (e) {
    ElMessage.error(e.message)
  }
}

const maxDaily = computed(() => Math.max(1, ...issues.value.daily.map((d) => Math.max(d.reported, d.closed))))
const curReported = computed(() => issues.value.daily.reduce((s, d) => s + (d.reported || 0), 0))
const curClosed = computed(() => issues.value.daily.reduce((s, d) => s + (d.closed || 0), 0))

function delta(cur, prev) {
  if (!prev) return cur ? '新增' : '0%'
  const p = Math.round(((cur - prev) / prev) * 100)
  return (p >= 0 ? '+' : '') + p + '%'
}

async function load() {
  loading.value = true
  try {
    const [l, w] = await Promise.all([
      api.get('/api/v1/reports/learning'),
      api.get('/api/v1/reports/wrong', { limit: 30 }),
    ])
    learning.value = l
    wrong.value = w
    await loadIssues()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div v-loading="loading">
    <el-tabs v-model="tab">
      <el-tab-pane label="学习档案" name="learning">
        <div class="cards">
          <div class="stat"><div class="n">{{ learning.users }}</div><div class="muted">员工数</div></div>
          <div class="stat"><div class="n">{{ learning.items }}</div><div class="muted">学习条目</div></div>
          <div class="stat"><div class="n">{{ learning.completed }}</div><div class="muted">累计完成次数</div></div>
        </div>
        <div style="margin-bottom:10px"><el-button size="small" @click="download('/api/v1/export/reports?type=learning', '学习档案.xlsx')">导出 Excel</el-button></div>
        <el-table :data="learning.rows" size="small" max-height="560">
          <el-table-column prop="name" label="姓名" width="100" />
          <el-table-column prop="dept" label="区队" width="120" />
          <el-table-column prop="completed" label="完成条目" width="100" />
          <el-table-column prop="hours" label="学时" width="80" />
          <el-table-column prop="last_at" label="最近学习" width="140" />
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="错题统计" name="wrong">
        <div class="cards">
          <div class="stat"><div class="n">{{ wrong.records }}</div><div class="muted">累计答错次数</div></div>
          <div class="stat"><div class="n">{{ wrong.users }}</div><div class="muted">涉及员工</div></div>
          <div class="stat"><div class="n">{{ wrong.rows.length }}</div><div class="muted">题目数</div></div>
        </div>
        <div style="margin-bottom:10px"><el-button size="small" @click="download('/api/v1/export/reports?type=wrong', '错题统计.xlsx')">导出 Excel</el-button></div>
        <el-table :data="wrong.rows" size="small" max-height="560">
          <el-table-column prop="stem" label="题目" min-width="320" show-overflow-tooltip />
          <el-table-column prop="job" label="岗位" width="100" />
          <el-table-column prop="category" label="分类" width="110" />
          <el-table-column prop="times" label="答错次数" width="90" />
          <el-table-column prop="users" label="人数" width="80" />
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="隐患趋势" name="issues">
        <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 10px">
          <span class="muted">统计周期</span>
          <el-radio-group v-model="issueDays" size="small" @change="loadIssues">
            <el-radio-button :value="7">近 7 天</el-radio-button>
            <el-radio-button :value="14">近 14 天</el-radio-button>
            <el-radio-button :value="30">近 30 天</el-radio-button>
          </el-radio-group>
        </div>

        <div class="cards">
          <div class="stat"><div class="n">{{ issues.total }}</div><div class="muted">累计问题总数</div></div>
          <div class="stat"><div class="n">{{ issues.closed }}</div><div class="muted">累计已闭环</div></div>
          <div class="stat"><div class="n">{{ issues.overdue }}</div><div class="muted">当前超时</div></div>
          <div class="stat"><div class="n">{{ Number(issues.avg_close_hours || 0).toFixed(1) }}</div><div class="muted">累计平均闭环小时</div></div>
        </div>

        <div class="muted" style="margin: 6px 0 12px">
          本期上报 <strong>{{ curReported }}</strong>（上期 {{ issues.prev_reported || 0 }}，环比 {{ delta(curReported, issues.prev_reported) }}）
          · 本期闭环 <strong>{{ curClosed }}</strong>（上期 {{ issues.prev_closed || 0 }}，环比 {{ delta(curClosed, issues.prev_closed) }}）
        </div>

        <div style="margin-bottom: 10px">
          <el-button size="small" @click="download(`/api/v1/export/reports?type=issues&days=${issueDays}`, '隐患趋势.xlsx')">导出趋势</el-button>
          <el-button size="small" @click="download('/api/v1/export/reports?type=dept', '区队隐患统计.xlsx')">导出区队统计</el-button>
        </div>

        <div class="panel">
          <div class="panel-title">近 {{ issueDays }} 天</div>
          <div v-for="d in issues.daily" :key="d.date" class="bar-row">
            <span class="bar-date">{{ d.date }}</span>
            <div class="bar-track"><div class="bar-in report" :style="{ width: (d.reported / maxDaily) * 100 + '%' }"></div></div>
            <span class="bar-num">{{ d.reported }}</span>
            <div class="bar-track"><div class="bar-in close" :style="{ width: (d.closed / maxDaily) * 100 + '%' }"></div></div>
            <span class="bar-num">{{ d.closed }}</span>
          </div>
          <div class="legend"><span class="dot report"></span>上报 <span class="dot close"></span>闭环</div>
        </div>

        <div class="panel">
          <div class="panel-title">按区队</div>
          <el-table :data="issues.by_dept" size="small">
            <el-table-column prop="dept" label="区队" min-width="140" />
            <el-table-column prop="total" label="总数" width="90" />
            <el-table-column prop="closed" label="已闭环" width="90" />
            <el-table-column prop="open" label="未闭环" width="90" />
          </el-table>
        </div>

        <div class="panel">
          <div class="panel-title">状态分布</div>
          <div class="row">
            <el-tag v-for="(n, k) in issues.status" :key="k" type="info" effect="plain" style="margin-right: 8px">
              {{ k }}：{{ n }}
            </el-tag>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<style scoped>
.cards {
  display: flex;
  gap: 12px;
  margin-bottom: 14px;
}
.stat {
  flex: 1;
  background: #fff;
  border: 1px solid #eef0f3;
  border-radius: 8px;
  padding: 14px 16px;
}
.n {
  font-size: 26px;
  font-weight: 700;
  color: #a5311f;
}
.panel {
  margin-bottom: 18px;
  background: #fff;
  border: 1px solid #eef0f3;
  border-radius: 8px;
  padding: 14px 16px;
}
.panel-title {
  font-weight: 600;
  margin-bottom: 10px;
}
.bar-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.bar-date {
  width: 48px;
  color: #8a94a0;
  font-size: 12px;
}
.bar-track {
  flex: 1;
  height: 10px;
  background: #f2f4f7;
  border-radius: 5px;
  overflow: hidden;
}
.bar-in {
  height: 100%;
}
.bar-in.report {
  background: #e0a21a;
}
.bar-in.close {
  background: #2f9e6e;
}
.bar-num {
  width: 24px;
  text-align: right;
  font-size: 12px;
  color: #59636e;
}
.legend {
  font-size: 12px;
  color: #8a94a0;
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 6px;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
  margin-left: 8px;
}
.dot.report {
  background: #e0a21a;
}
.dot.close {
  background: #2f9e6e;
}
</style>
