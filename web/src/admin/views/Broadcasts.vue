<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../../shared/http'
import { broadcastStatus, fmtTime } from '../../shared/format'

const rows = ref([])
const total = ref(0)
const loading = ref(false)
const page = reactive({ page: 1, page_size: 20 })

const depts = ref([])
const dialog = ref(false)
const form = reactive({ title: '', content: '', dept_ids: [], require_ack: true, deadline: null, publish: true })

const recordsDialog = ref(false)
const records = ref([])
const current = ref(null)

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/v1/broadcast/tasks', { ...page })
    rows.value = data.list || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await load()
  try {
    depts.value = await api.get('/api/v1/depts')
  } catch {
    depts.value = []
  }
})

function openCreate() {
  Object.assign(form, { title: '', content: '', dept_ids: [], require_ack: true, deadline: null, publish: true })
  dialog.value = true
}

async function save() {
  if (!form.title.trim() || !form.content.trim()) {
    ElMessage.warning('标题与内容必填')
    return
  }
  try {
    await api.post('/api/v1/broadcast/tasks', {
      title: form.title,
      content: form.content,
      dept_ids: form.dept_ids,
      require_ack: form.require_ack,
      deadline: form.deadline ? new Date(form.deadline).toISOString() : null,
      publish: form.publish,
    })
    ElMessage.success(form.publish ? '已发布并生成签收记录' : '已保存草稿')
    dialog.value = false
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function publish(row) {
  try {
    await api.post(`/api/v1/broadcast/tasks/${row.id}/publish`)
    ElMessage.success('已发布')
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function openRecords(row) {
  current.value = row
  recordsDialog.value = true
  const data = await api.get(`/api/v1/broadcast/tasks/${row.id}/records`, { page_size: 200 })
  records.value = data.list || []
}
</script>

<template>
  <div class="panel">
    <div style="display: flex; gap: 10px; margin-bottom: 14px">
      <el-button type="primary" @click="openCreate">新建宣贯</el-button>
      <div style="flex: 1"></div>
      <span class="muted">共 {{ total }} 条</span>
    </div>

    <el-table v-loading="loading" :data="rows" size="small">
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column prop="title" label="标题" min-width="220" show-overflow-tooltip />
      <el-table-column label="状态" width="96">
        <template #default="{ row }">
          <el-tag :type="row.status === 'published' ? 'success' : 'info'" size="small">{{ broadcastStatus[row.status] }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="签收" width="130">
        <template #default="{ row }">
          <span>{{ row.ack_count }} / {{ row.total_count }}</span>
        </template>
      </el-table-column>
      <el-table-column label="截止" width="150">
        <template #default="{ row }">{{ row.deadline ? fmtTime(row.deadline) : '—' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="170">
        <template #default="{ row }">
          <el-button v-if="row.status === 'draft'" link type="success" @click="publish(row)">发布</el-button>
          <el-button link type="primary" @click="openRecords(row)">签收明细</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" title="新建宣贯任务" width="560px">
      <el-form label-width="88px">
        <el-form-item label="标题"><el-input v-model="form.title" /></el-form-item>
        <el-form-item label="内容"><el-input v-model="form.content" type="textarea" :rows="4" /></el-form-item>
        <el-form-item label="范围">
          <el-select v-model="form.dept_ids" multiple placeholder="不选表示全员" style="width: 100%">
            <el-option v-for="d in depts" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="截止时间">
          <el-date-picker v-model="form.deadline" type="datetime" style="width: 100%" />
        </el-form-item>
        <el-form-item label="需签收"><el-switch v-model="form.require_ack" /></el-form-item>
        <el-form-item label="立即发布"><el-switch v-model="form.publish" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="recordsDialog" :title="`签收明细 · ${current?.title || ''}`" size="520px">
      <el-table :data="records" size="small">
        <el-table-column prop="user_name" label="姓名" width="90" />
        <el-table-column prop="dept_name" label="部门" width="120" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'ack' ? 'success' : row.status === 'read' ? 'warning' : 'info'" size="small">
              {{ row.status === 'ack' ? '已签收' : row.status === 'read' ? '已读' : '未读' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="签收时间" width="150">
          <template #default="{ row }">{{ fmtTime(row.ack_at) }}</template>
        </el-table-column>
      </el-table>
    </el-drawer>
  </div>
</template>
