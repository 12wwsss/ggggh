<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../../shared/http'

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

onMounted(load)

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
    const data = await api.get('/api/v1/quiz/items/' + itemId + '/questions')
    questions.value = data.list || data || []
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
    <div style="display: flex; margin-bottom: 14px">
      <el-button type="primary" @click="openCreate">新建学习条目</el-button>
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
      <el-table-column label="操作" width="210">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
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

    <el-dialog v-model="dialog" :title="form.id ? '编辑学习条目' : '新建学习条目'" width="640px">
      <el-form label-width="88px">
        <el-form-item label="标题"><el-input v-model="form.title" /></el-form-item>
        <el-form-item label="分类"><el-input v-model="form.category" /></el-form-item>
        <el-form-item label="适用岗位"><el-input v-model="form.post" placeholder="留空表示全部岗位" /></el-form-item>
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
