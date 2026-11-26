<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api, download } from '../../shared/http'

const rows = ref([])
const total = ref(0)
const loading = ref(false)
const page = reactive({ page: 1, page_size: 20 })

const statusText = { published: '已发布', draft: '草稿', archived: '归档' }

const dialog = ref(false)
const form = reactive({ id: 0, title: '', category: '规程', post: '', content: '', clause_refs: '', sort: 0, status: 'published' })

const quizDialog = ref(false)
const quizItem = ref(null)
const questions = ref([])
const qForm = reactive({ stem: '', options: '', answer: 'A', analysis: '' })

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/v1/learn/items', { ...page })
    rows.value = data.list || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

const assignVisible = ref(false)
const assignments = ref([])
const assignItem = ref(null)
const depts = ref([])
const assignForm = reactive({ post: '', dept_id: null, due_at: '' })

async function loadDepts() {
  try {
    const data = await api.get('/api/v1/depts')
    depts.value = data.list || data || []
  } catch (e) {
    depts.value = []
  }
}

async function openAssign(row) {
  assignItem.value = row
  assignForm.post = ''
  assignForm.dept_id = null
  assignForm.due_at = ''
  assignVisible.value = true
  await loadAssignments(row.id)
}

async function loadAssignments(itemId) {
  try {
    const data = await api.get('/api/v1/learn/assignments', { item_id: itemId })
    assignments.value = data.list || []
  } catch (e) {
    assignments.value = []
  }
}

async function saveAssign() {
  if (!assignItem.value) return
  try {
    await api.post('/api/v1/learn/assignments', {
      item_id: assignItem.value.id,
      post: assignForm.post,
      dept_id: assignForm.dept_id,
      due_at: assignForm.due_at,
    })
    ElMessage.success('已分配')
    await loadAssignments(assignItem.value.id)
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function delAssign(row) {
  try {
    await api.del('/api/v1/learn/assignments/' + row.id)
    await loadAssignments(assignItem.value.id)
  } catch (e) {
    ElMessage.error(e.message)
  }
}

const posts = ref([])

async function loadPosts() {
  try {
    const data = await api.get('/api/v1/posts')
    posts.value = (data.list || []).filter((p) => p.status === 'active').map((p) => p.name)
  } catch (e) {
    posts.value = []
  }
}

onMounted(load)
onMounted(loadDepts)
onMounted(loadPosts)

function openCreate() {
  Object.assign(form, { id: 0, title: '', category: '规程', post: '', content: '', clause_refs: '', sort: 0, status: 'published' })
  dialog.value = true
}

function openEdit(row) {
  Object.assign(form, {
    ...row,
    clause_refs: (row.clause_refs || []).join('、'),
  })
  dialog.value = true
}

async function save() {
  if (!form.title.trim() || !form.content.trim()) {
    ElMessage.warning('标题与内容必填')
    return
  }
  const payload = {
    title: form.title,
    category: form.category,
    post: form.post,
    content: form.content,
    clause_refs: form.clause_refs
      ? form.clause_refs.split(/[、,，\s]+/).filter(Boolean)
      : [],
    sort: Number(form.sort) || 0,
    status: form.status,
  }
  try {
    if (form.id) await api.put(`/api/v1/learn/items/${form.id}`, payload)
    else await api.post('/api/v1/learn/items', payload)
    ElMessage.success('已保存')
    dialog.value = false
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function openQuiz(row) {
  quizItem.value = row
  quizDialog.value = true
  await loadQuestions(row.id)
  Object.assign(qForm, { stem: '', options: '', answer: 'A', analysis: '' })
}

async function loadQuestions(itemId) {
  try {
    const data = await api.get('/api/v1/quiz/questions?item_id=' + itemId)
    questions.value = data.list || data || []
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function reviewQuestion(row, status) {
  try {
    await api.post(`/api/v1/quiz/questions/${row.id}/status`, { status })
    row.status = status
    ElMessage.success(status === 'published' ? '已通过' : '已驳回')
  } catch (e) {
    ElMessage.error(e.message)
  }
}
async function addQuestion() {
  if (!qForm.stem.trim() || !qForm.options.trim()) {
    ElMessage.warning('题干与选项必填')
    return
  }
  const options = qForm.options
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean)
  if (options.length < 2) {
    ElMessage.warning('至少两个选项')
    return
  }
  try {
    await api.post('/api/v1/quiz/questions?item_id=' + quizItem.value.id, {
      stem: qForm.stem,
      options,
      answer: qForm.answer,
      analysis: qForm.analysis,
      difficulty: 'normal',
    })
    ElMessage.success('已添加')
    Object.assign(qForm, { stem: '', options: '', answer: 'A', analysis: '' })
    await loadQuestions(quizItem.value.id)
  } catch (e) {
    ElMessage.error(e.message)
  }
}

const importVisible = ref(false)
const importBusy = ref(false)
const importFile = ref(null)

function onImportFile(f) {
  importFile.value = f.raw
}

async function doImport() {
  if (!importFile.value) return ElMessage.warning('请选择 CSV 文件')
  importBusy.value = true
  try {
    const res = await api.upload('/api/v1/import/quiz', importFile.value)
    ElMessage.success('导入完成：成功 ' + (res.created || 0) + ' 条，跳过 ' + (res.skipped || 0) + ' 条（待审核）')
    importVisible.value = false
    importFile.value = null
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    importBusy.value = false
  }
}
async function toggleRequired(row, v) {
  try {
    await api.put(`/api/v1/learn/items/${row.id}`, { ...row, required: v })
    row.required = v
    ElMessage.success(v ? '已设为必修' : '已取消必修')
  } catch (e) {
    ElMessage.error(e.message)
  }
}
async function removeItem(row) {
  try {
    await ElMessageBox.confirm('删除后学习记录保留但不再展示，确定？', '提示', { type: 'warning' })
    await api.put(`/api/v1/learn/items/${row.id}`, { ...row, status: 'archived' })
    ElMessage.success('已归档')
    await load()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(e.message)
  }
}
</script>

<template>
  <div class="panel">
    <div style="display: flex; gap: 8px; margin-bottom: 14px">
      <el-button type="primary" @click="openCreate">新建学习条目</el-button>
      <el-button @click="download('/api/v1/export/quiz', '题库.xlsx')">导出题库</el-button>
      <el-button @click="importVisible = true">导入题库（CSV）</el-button>
      <div style="flex: 1"></div>
      <span class="muted">共 {{ total }} 条</span>
    </div>

    <el-table v-loading="loading" :data="rows" size="small">
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column prop="title" label="标题" min-width="220" show-overflow-tooltip />
      <el-table-column prop="category" label="分类" width="110" />
      <el-table-column prop="post" label="适用岗位" width="120" />
      <el-table-column label="关联条款" min-width="180">
        <template #default="{ row }">
          <span v-for="r in row.clause_refs" :key="r" class="clause-chip">{{ r }}</span>
          <span v-if="!row.clause_refs?.length" class="muted">—</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 'published' ? 'success' : 'info'" size="small">
            {{ statusText[row.status] || row.status }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="必修" width="80">
        <template #default="{ row }">
          <el-switch :model-value="row.required" @change="(v) => toggleRequired(row, v)" />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="210">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="info" @click="openAssign(row)">分配</el-button>
          <el-button link type="success" @click="openQuiz(row)">题目</el-button>
          <el-button link type="warning" @click="removeItem(row)">归档</el-button>
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

    <el-dialog v-model="assignVisible" :title="`学习计划分配 · ${assignItem?.title || ''}`" width="620px">
      <el-table :data="assignments" size="small" max-height="240">
        <el-table-column prop="post" label="岗位" width="100">
          <template #default="{ row }">{{ row.post || '全部' }}</template>
        </el-table-column>
        <el-table-column prop="dept_name" label="区队" width="120">
          <template #default="{ row }">{{ row.dept_name || '全部' }}</template>
        </el-table-column>
        <el-table-column prop="due_at" label="截止" width="120" />
        <el-table-column label="操作" width="80">
          <template #default="{ row }">
            <el-button link type="danger" @click="delAssign(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-divider content-position="left">新增分配</el-divider>
      <el-form label-width="80px">
        <el-form-item label="岗位">
          <el-select v-model="assignForm.post" filterable allow-create clearable placeholder="留空表示全部岗位" style="width: 100%">
            <el-option v-for="p in posts" :key="p" :label="p" :value="p" />
          </el-select>
        </el-form-item>
        <el-form-item label="区队">
          <el-select v-model="assignForm.dept_id" clearable placeholder="留空表示全部区队" style="width: 100%">
            <el-option v-for="d in depts" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="截止日期">
          <el-date-picker v-model="assignForm.due_at" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="assignVisible = false">关闭</el-button>
        <el-button type="primary" @click="saveAssign">分配</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="importVisible" title="导入题库（CSV）" width="560px">
      <div class="muted" style="line-height: 1.8; margin-bottom: 10px">
        表头必须包含：岗位、分类、题干、选项A、选项B、选项C、选项D、答案、解析。<br />
        答案填 A/B/C/D。导入的题目为「待审核」，通过后才进入员工练习。
      </div>
      <el-upload :auto-upload="false" :limit="1" accept=".csv" :on-change="onImportFile">
        <el-button>选择 CSV 文件</el-button>
      </el-upload>
      <template #footer>
        <el-button @click="importVisible = false">取消</el-button>
        <el-button type="primary" :loading="importBusy" @click="doImport">开始导入</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="dialog" :title="form.id ? '编辑学习条目' : '新建学习条目'" width="640px">
      <el-form label-width="88px">
        <el-form-item label="标题"><el-input v-model="form.title" /></el-form-item>
        <el-form-item label="分类"><el-input v-model="form.category" /></el-form-item>
        <el-form-item label="适用岗位">
          <el-select v-model="form.post" filterable allow-create clearable placeholder="留空表示全部岗位" style="width: 100%">
            <el-option v-for="p in posts" :key="p" :label="p" :value="p" />
          </el-select>
        </el-form-item>
        <el-form-item label="关联条款">
          <el-input v-model="form.clause_refs" placeholder="用顿号分隔，如：第五十五条、第五十六条" />
        </el-form-item>
        <el-form-item label="内容">
          <el-input v-model="form.content" type="textarea" :rows="6" />
        </el-form-item>
        <el-form-item label="排序"><el-input-number v-model="form.sort" :min="0" /></el-form-item>
        <el-form-item label="状态">
          <el-select v-model="form.status" style="width: 160px">
            <el-option label="已发布" value="published" />
            <el-option label="草稿" value="draft" />
            <el-option label="归档" value="archived" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="quizDialog" :title="`题目管理 · ${quizItem?.title || ''}`" width="720px">
      <el-table :data="questions" size="small" max-height="280">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="stem" label="题干" min-width="240" show-overflow-tooltip />
        <el-table-column prop="answer" label="答案" width="70" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'published' ? 'success' : row.status === 'pending' ? 'warning' : 'danger'" size="small">
              {{ row.status === 'published' ? '已通过' : row.status === 'pending' ? '待审核' : '已驳回' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="审核" width="130">
          <template #default="{ row }">
            <el-button v-if="row.status !== 'published'" link type="success" @click="reviewQuestion(row, 'published')">通过</el-button>
            <el-button v-if="row.status !== 'rejected'" link type="danger" @click="reviewQuestion(row, 'rejected')">驳回</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-divider content-position="left">新增题目</el-divider>
      <el-form label-width="72px">
        <el-form-item label="题干"><el-input v-model="qForm.stem" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="选项">
          <el-input v-model="qForm.options" type="textarea" :rows="4" placeholder="每行一个选项，如：A. 敲帮问顶" />
        </el-form-item>
        <el-form-item label="答案"><el-input v-model="qForm.answer" style="width: 120px" /></el-form-item>
        <el-form-item label="解析"><el-input v-model="qForm.analysis" type="textarea" :rows="2" /></el-form-item>
        <el-form-item>
          <el-button type="primary" @click="addQuestion">添加题目</el-button>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="quizDialog = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>
