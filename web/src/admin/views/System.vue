<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../../shared/http'
import { fmtTime, taskStatus } from '../../shared/format'

const tab = ref('tasks')

const kindText = {
  doc_parse: '文档解析',
  doc_embed: '向量生成',
  issue_overdue_sweep: '问题超时扫描',
  broadcast_deadline_scan: '宣贯到期扫描',
}

const tasks = ref([])
const taskTotal = ref(0)
const taskPage = reactive({ page: 1, page_size: 20, status: '' })

const audits = ref([])
const auditTotal = ref(0)
const auditPage = reactive({ page: 1, page_size: 20, resource: '', action: '' })

async function loadTasks() {
  try {
    const params = { page: taskPage.page, page_size: taskPage.page_size }
    if (taskPage.status) params.status = taskPage.status
    const data = await api.get('/api/v1/tasks', params)
    tasks.value = data.list || []
    taskTotal.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function loadAudits() {
  try {
    const data = await api.get('/api/v1/audit', { ...auditPage })
    audits.value = data.list || []
    auditTotal.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(async () => {
  await loadTasks()
  await loadAudits()
})

async function retry(row) {
  try {
    await api.post(`/api/v1/tasks/${row.id}/retry`)
    ElMessage.success('已重新排队')
    await loadTasks()
  } catch (e) {
    ElMessage.error(e.message)
  }
}
</script>

<template>
  <div class="panel">
    <el-tabs v-model="tab">
      <el-tab-pane label="后台任务" name="tasks">
        <div style="display: flex; gap: 10px; margin-bottom: 12px">
          <el-select v-model="taskPage.status" placeholder="全部状态" clearable style="width: 150px" @change="(taskPage.page = 1), loadTasks()">
            <el-option label="排队中" value="pending" />
            <el-option label="执行中" value="running" />
            <el-option label="完成" value="succeeded" />
            <el-option label="失败" value="failed" />
          </el-select>
          <el-button @click="loadTasks">刷新</el-button>
          <div style="flex: 1"></div>
          <span class="muted">共 {{ taskTotal }} 条</span>
        </div>
        <el-table :data="tasks" size="small">
          <el-table-column prop="id" label="ID" width="64" />
          <el-table-column label="类型" min-width="150">
            <template #default="{ row }">{{ kindText[row.kind] || row.kind }}</template>
          </el-table-column>
          <el-table-column label="状态" width="96">
            <template #default="{ row }">
              <el-tag :type="taskStatus[row.status]?.type || 'info'" size="small">{{ taskStatus[row.status]?.text }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="尝试" width="80">
            <template #default="{ row }">{{ row.attempts }} / {{ row.max_attempts }}</template>
          </el-table-column>
          <el-table-column prop="last_error" label="最近错误" min-width="240" show-overflow-tooltip />
          <el-table-column label="创建时间" width="150">
            <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="90">
            <template #default="{ row }">
              <el-button v-if="row.status !== 'running'" link type="primary" @click="retry(row)">重试</el-button>
            </template>
          </el-table-column>
        </el-table>
        <div style="margin-top: 12px; display: flex; justify-content: flex-end">
          <el-pagination
            v-model:current-page="taskPage.page"
            v-model:page-size="taskPage.page_size"
            :total="taskTotal"
            layout="total, prev, pager, next"
            @current-change="loadTasks"
          />
        </div>
      </el-tab-pane>

      <el-tab-pane label="审计日志" name="audit">
        <div style="display: flex; gap: 10px; margin-bottom: 12px">
          <el-input v-model="auditPage.resource" placeholder="资源，如 doc" clearable style="width: 160px" @change="(auditPage.page = 1), loadAudits()" />
          <el-input v-model="auditPage.action" placeholder="动作，如 post" clearable style="width: 160px" @change="(auditPage.page = 1), loadAudits()" />
          <el-button @click="loadAudits">刷新</el-button>
          <div style="flex: 1"></div>
          <span class="muted">共 {{ auditTotal }} 条</span>
        </div>
        <el-table :data="audits" size="small">
          <el-table-column prop="id" label="ID" width="64" />
          <el-table-column label="时间" width="150">
            <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column prop="username" label="操作人" width="110" />
          <el-table-column prop="action" label="动作" width="90" />
          <el-table-column prop="resource" label="资源" width="110" />
          <el-table-column prop="resource_id" label="对象" width="90" />
          <el-table-column prop="ip" label="IP" width="130" />
          <el-table-column label="详情" min-width="280" show-overflow-tooltip>
            <template #default="{ row }">{{ JSON.stringify(row.detail) }}</template>
          </el-table-column>
        </el-table>
        <div style="margin-top: 12px; display: flex; justify-content: flex-end">
          <el-pagination
            v-model:current-page="auditPage.page"
            v-model:page-size="auditPage.page_size"
            :total="auditTotal"
            layout="total, prev, pager, next"
            @current-change="loadAudits"
          />
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>
