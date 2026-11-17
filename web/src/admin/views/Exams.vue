<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api, download } from '../../shared/http'
import { examStatus, fmtTime } from '../../shared/format'

const rows = ref([])
const total = ref(0)
const loading = ref(false)
const page = reactive({ page: 1, page_size: 20 })

const allQuestions = ref([])
const dialog = ref(false)
const form = reactive({ title: '', question_ids: [], pass_score: 60, duration_min: 60, max_attempts: 1 })

const recordsDialog = ref(false)
const records = ref([])
const currentPaper = ref(null)

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

onMounted(async () => {
  await load()
  await loadQuestions()
})

function openCreate() {
  Object.assign(form, { title: '', question_ids: [], pass_score: 60, duration_min: 60, max_attempts: 1 })
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
    })
    ElMessage.success('已创建草稿试卷')
    dialog.value = false
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function publish(row) {
  try {
    const { value } = await ElMessageBox.prompt(
      `发布《${row.title}》后员工即可参加。可考次数（1–10），员工用完后不能再进入：`,
      '确认发布',
      {
        inputValue: String(row.max_attempts || 1),
        inputPattern: /^([1-9]|10)$/,
        inputErrorMessage: '请输入 1–10 的整数',
        type: 'warning',
      },
    )
    await api.post(`/api/v1/exam/papers/${row.id}/publish`, { max_attempts: Number(value) })
    ElMessage.success('已发布')
    await load()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(e.message)
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
          <el-button v-if="row.status === 'draft'" link type="success" @click="publish(row)">发布</el-button>
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
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
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
