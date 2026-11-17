<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api, download } from '../../shared/http'
import { examStatus, fmtTime } from '../../shared/format'

const rows = ref([])
const total = ref(0)
const loading = ref(false)
const page = reactive({ page: 1, page_size: 20 })

const allQuestions = ref([])
const depts = ref([])
const dialog = ref(false)
const form = reactive({ title: '', question_ids: [], pass_score: 60, duration_min: 60, max_attempts: 1, dept_ids: [] })

const publishDialog = ref(false)
const publishForm = reactive({ id: 0, title: '', max_attempts: 1, dept_ids: [] })

const recordsDialog = ref(false)
const records = ref([])
const currentPaper = ref(null)

function deptText(ids) {
  if (!ids || !ids.length) return '全员'
  return ids.map((id) => depts.value.find((d) => d.id === id)?.name || `#${id}`).join('、')
}

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/v1/exam/papers', { ...page })
    rows.value = data.list || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function loadQuestions() {
  try {
    const data = await api.get('/api/v1/quiz/questions')
    allQuestions.value = data.list || data || []
  } catch (e) {
    allQuestions.value = []
  }
}

async function loadDepts() {
  try {
    depts.value = (await api.get('/api/v1/depts')) || []
  } catch {
    depts.value = []
  }
}

onMounted(async () => {
  await load()
  await loadQuestions()
  await loadDepts()
})

function openCreate() {
  Object.assign(form, {
    title: '',
    question_ids: [],
    pass_score: 60,
    duration_min: 60,
    max_attempts: 1,
    dept_ids: [],
  })
  dialog.value = true
}

async function save() {
  if (!form.title.trim() || !form.question_ids.length) {
    ElMessage.warning('标题与题目必填')
    return
  }
  try {
    await api.post('/api/v1/exam/papers', {
      title: form.title,
      question_ids: form.question_ids,
      pass_score: Number(form.pass_score),
      duration_min: Number(form.duration_min),
      max_attempts: Number(form.max_attempts),
      dept_ids: form.dept_ids,
    })
    ElMessage.success('已创建草稿试卷')
    dialog.value = false
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function openPublish(row) {
  publishForm.id = row.id
  publishForm.title = row.title
  publishForm.max_attempts = row.max_attempts || 1
  publishForm.dept_ids = [...(row.dept_ids || [])]
  publishDialog.value = true
}

async function confirmPublish() {
  try {
    await api.post(`/api/v1/exam/papers/${publishForm.id}/publish`, {
      max_attempts: Number(publishForm.max_attempts),
      dept_ids: publishForm.dept_ids,
    })
    ElMessage.success('已发布')
    publishDialog.value = false
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function openRecords(row) {
  currentPaper.value = row
  recordsDialog.value = true
  const data = await api.get('/api/v1/exam/records', { paper_id: row.id, page_size: 100 })
  records.value = data.list || []
}

function exportRecords() {
  download('/api/v1/exam/export', 'exam-records.csv').catch((e) => ElMessage.error(e.message))
}
</script>

<template>
  <div class="panel">
    <div style="display: flex; gap: 10px; margin-bottom: 14px">
      <el-button type="primary" @click="openCreate">新建试卷</el-button>
      <el-button @click="exportRecords">导出成绩</el-button>
      <div style="flex: 1"></div>
      <span class="muted">共 {{ total }} 份</span>
    </div>

    <el-table v-loading="loading" :data="rows" size="small">
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column prop="title" label="试卷" min-width="220" />
      <el-table-column label="题目数" width="90">
        <template #default="{ row }">{{ row.question_ids?.length || 0 }}</template>
      </el-table-column>
      <el-table-column prop="pass_score" label="及格" width="72" />
      <el-table-column prop="duration_min" label="时长(分)" width="90" />
      <el-table-column prop="max_attempts" label="可考次数" width="90" />
      <el-table-column label="发布部门" min-width="140">
        <template #default="{ row }">{{ deptText(row.dept_ids) }}</template>
      </el-table-column>
      <el-table-column label="状态" width="96">
        <template #default="{ row }">
          <el-tag :type="row.status === 'published' ? 'success' : 'info'" size="small">{{ examStatus[row.status] }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="150">
        <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <el-button v-if="row.status === 'draft'" link type="success" @click="openPublish(row)">发布</el-button>
          <el-button link type="primary" @click="openRecords(row)">成绩</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" title="新建试卷" width="620px">
      <el-form label-width="88px">
        <el-form-item label="试卷标题"><el-input v-model="form.title" /></el-form-item>
        <el-form-item label="选择题目">
          <el-select v-model="form.question_ids" multiple filterable style="width: 100%" placeholder="从题库中选择">
            <el-option v-for="q in allQuestions" :key="q.id" :label="`#${q.id} ${q.stem}`" :value="q.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="及格分">
          <el-input-number v-model="form.pass_score" :min="1" :max="100" />
        </el-form-item>
        <el-form-item label="时长(分)">
          <el-input-number v-model="form.duration_min" :min="5" :max="300" />
        </el-form-item>
        <el-form-item label="可考次数">
          <el-input-number v-model="form.max_attempts" :min="1" :max="10" />
          <span class="muted" style="margin-left: 8px">发布时还可以再改</span>
        </el-form-item>
        <el-form-item label="发布部门">
          <el-select v-model="form.dept_ids" multiple filterable style="width: 100%" placeholder="不选表示全员">
            <el-option v-for="d in depts" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="publishDialog" title="发布考试" width="480px">
      <el-form label-width="88px">
        <el-form-item label="试卷">{{ publishForm.title }}</el-form-item>
        <el-form-item label="发布部门">
          <el-select v-model="publishForm.dept_ids" multiple filterable style="width: 100%" placeholder="不选表示全员">
            <el-option v-for="d in depts" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="可考次数">
          <el-input-number v-model="publishForm.max_attempts" :min="1" :max="10" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="publishDialog = false">取消</el-button>
        <el-button type="primary" @click="confirmPublish">确认发布</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="recordsDialog" :title="`成绩 · ${currentPaper?.title || ''}`" size="560px">
      <el-table :data="records" size="small">
        <el-table-column prop="user_name" label="姓名" width="90" />
        <el-table-column prop="dept_name" label="部门" width="110" />
        <el-table-column prop="score" label="得分" width="72" />
        <el-table-column prop="attempt_no" label="第几次" width="76" />
        <el-table-column label="结果" width="86">
          <template #default="{ row }">
            <el-tag :type="row.passed ? 'success' : 'danger'" size="small">{{ row.passed ? '通过' : '未通过' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="交卷" width="150">
          <template #default="{ row }">{{ fmtTime(row.submitted_at) }}</template>
        </el-table-column>
      </el-table>
    </el-drawer>
  </div>
</template>
