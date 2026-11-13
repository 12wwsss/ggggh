<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../../shared/http'
import { fmtTime } from '../../shared/format'

const rows = ref([])
const total = ref(0)
const loading = ref(false)
const answered = ref('')
const page = reactive({ page: 1, page_size: 20 })

const drawer = ref(false)
const current = ref(null)

async function load() {
  loading.value = true
  try {
    const params = { ...page }
    if (answered.value !== '') params.answered = answered.value
    const data = await api.get('/api/v1/qa/logs', params)
    rows.value = data.list || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)

function open(row) {
  current.value = row
  drawer.value = true
}

async function feedback(value) {
  try {
    await api.post(`/api/v1/qa/logs/${current.value.id}/feedback`, { feedback: value })
    current.value.feedback = value
    ElMessage.success('已记录反馈')
  } catch (e) {
    ElMessage.error(e.message)
  }
}
</script>

<template>
  <div class="panel">
    <div style="display: flex; gap: 10px; margin-bottom: 14px">
      <el-select v-model="answered" placeholder="全部" clearable style="width: 160px" @change="(page.page = 1), load()">
        <el-option label="已回答" value="true" />
        <el-option label="走兜底" value="false" />
      </el-select>
      <el-button @click="load">刷新</el-button>
      <div style="flex: 1"></div>
      <span class="muted">共 {{ total }} 条</span>
    </div>

    <el-table v-loading="loading" :data="rows" size="small">
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column prop="question" label="问题" min-width="280" show-overflow-tooltip />
      <el-table-column prop="user_name" label="提问人" width="96" />
      <el-table-column label="结果" width="90">
        <template #default="{ row }">
          <el-tag :type="row.answered ? 'success' : 'info'" size="small">{{ row.answered ? '已回答' : '兜底' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="最高分" width="90">
        <template #default="{ row }">{{ Number(row.top_score || 0).toFixed(3) }}</template>
      </el-table-column>
      <el-table-column prop="latency_ms" label="耗时" width="80">
        <template #default="{ row }">{{ row.latency_ms }}ms</template>
      </el-table-column>
      <el-table-column label="时间" width="150">
        <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="80">
        <template #default="{ row }">
          <el-button link type="primary" @click="open(row)">详情</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div style="margin-top: 14px; display: flex; justify-content: flex-end">
      <el-pagination
        v-model:current-page="page.page"
        v-model:page-size="page.page_size"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="load"
      />
    </div>

    <el-drawer v-model="drawer" title="问答详情" size="520px">
      <template v-if="current">
        <h4 style="margin: 0 0 6px">问题</h4>
        <p style="margin: 0 0 16px">{{ current.question }}</p>
        <h4 style="margin: 0 0 6px">回答</h4>
        <p style="white-space: pre-wrap; line-height: 1.8; margin-top: 0">{{ current.answer }}</p>
        <h4 style="margin: 16px 0 6px">引用</h4>
        <div v-if="!current.citations?.length" class="muted">无引用（走兜底）</div>
        <el-tag v-for="c in current.citations" :key="c.chunk_id" style="margin: 0 8px 8px 0">
          {{ c.clause_no }} · 片段 {{ c.chunk_id }}
        </el-tag>
        <el-descriptions :column="2" border size="small" style="margin-top: 16px">
          <el-descriptions-item label="提问人">{{ current.user_name }}</el-descriptions-item>
          <el-descriptions-item label="模型">{{ current.model }}</el-descriptions-item>
          <el-descriptions-item label="最高相似度">{{ Number(current.top_score || 0).toFixed(3) }}</el-descriptions-item>
          <el-descriptions-item label="命中条数">{{ current.hit_count }}</el-descriptions-item>
          <el-descriptions-item label="IP">{{ current.ip || '—' }}</el-descriptions-item>
          <el-descriptions-item label="时间">{{ fmtTime(current.created_at) }}</el-descriptions-item>
        </el-descriptions>
        <h4 style="margin: 16px 0 6px">反馈</h4>
        <el-button size="small" :type="current.feedback === 'useful' ? 'success' : ''" @click="feedback('useful')">有用</el-button>
        <el-button size="small" :type="current.feedback === 'useless' ? 'warning' : ''" @click="feedback('useless')">没用</el-button>
        <el-button size="small" :type="current.feedback === 'wrong_clause' ? 'danger' : ''" @click="feedback('wrong_clause')">
          条款不对
        </el-button>
      </template>
    </el-drawer>
  </div>
</template>
