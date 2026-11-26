<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../../shared/http'
import { fmtTime } from '../../shared/format'

const KIND = { article: '图文', culture: '安全文化', notice: '宣贯解读' }
const STATUS = { draft: '草稿', published: '已发布', archived: '已下架' }
const CATS = ['案例', '安全月', '寄语', '光荣榜', '知识卡', '规程宣贯', '警示教育']

const rows = ref([])
const loading = ref(false)
const filter = reactive({ kind: '', status: '' })
const dialog = ref(false)
const saving = ref(false)
const form = reactive({
  id: '',
  kind: 'article',
  cat: '案例',
  title: '',
  tag: '',
  summary: '',
  body: '',
  date: '',
  minutes: 2,
  ask: '',
  sort: 0,
  status: 'published',
})

const dialogTitle = computed(() => (form.id ? '编辑内容' : '新建内容'))

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/v1/content', { kind: filter.kind, status: filter.status })
    rows.value = data.list || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function reset() {
  Object.assign(form, {
    id: '',
    kind: 'article',
    cat: '案例',
    title: '',
    tag: '',
    summary: '',
    body: '',
    date: new Date().toISOString().slice(0, 10),
    minutes: 2,
    ask: '',
    sort: 0,
    status: 'published',
  })
}

function openCreate() {
  reset()
  dialog.value = true
}

function openEdit(row) {
  const p = row.payload || {}
  Object.assign(form, {
    id: row.id,
    kind: row.kind,
    cat: row.cat || p.cat || '',
    title: row.title || p.title || '',
    tag: p.tag || '',
    summary: p.summary || '',
    body: Array.isArray(p.body) ? p.body.join('\n') : p.body || '',
    date: p.date || '',
    minutes: p.minutes || 0,
    ask: p.ask || '',
    sort: row.sort || 0,
    status: row.status || 'published',
  })
  dialog.value = true
}

async function save() {
  if (!form.title.trim()) return ElMessage.warning('标题必填')
  if (!form.body.trim()) return ElMessage.warning('正文不能为空')
  saving.value = true
  try {
    await api.post('/api/v1/content', { ...form })
    ElMessage.success('已保存')
    dialog.value = false
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function toggleStatus(row) {
  const next = row.status === 'published' ? 'archived' : 'published'
  try {
    await api.post('/api/v1/content', {
      id: row.id,
      kind: row.kind,
      cat: row.cat,
      title: row.title,
      sort: row.sort,
      status: next,
      body: Array.isArray(row.payload?.body) ? row.payload.body.join('\n') : row.payload?.body || '',
      tag: row.payload?.tag || '',
      summary: row.payload?.summary || '',
      date: row.payload?.date || '',
      minutes: row.payload?.minutes || 0,
      ask: row.payload?.ask || '',
    })
    ElMessage.success(next === 'published' ? '已上架' : '已下架')
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function remove(row) {
  try {
    await ElMessageBox.confirm(`删除「${row.title}」？`, '确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await api.del(`/api/v1/content/${row.id}`)
    ElMessage.success('已删除')
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="toolbar">
      <div class="muted">
        这里维护员工端首页「安全文化」和图文详情。发布后小程序与电脑端同时可见；下架后立即隐藏。
      </div>
      <div class="acts">
        <el-select v-model="filter.kind" placeholder="全部类型" clearable style="width: 130px" @change="load">
          <el-option v-for="(label, k) in KIND" :key="k" :label="label" :value="k" />
        </el-select>
        <el-select v-model="filter.status" placeholder="全部状态" clearable style="width: 120px" @change="load">
          <el-option v-for="(label, k) in STATUS" :key="k" :label="label" :value="k" />
        </el-select>
        <el-button type="primary" @click="openCreate">新建内容</el-button>
      </div>
    </div>

    <el-table :data="rows" v-loading="loading" size="small">
      <el-table-column label="类型" width="100">
        <template #default="{ row }">
          <el-tag size="small" :type="row.kind === 'culture' ? 'warning' : 'primary'">{{ KIND[row.kind] }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="cat" label="分类" width="100" />
      <el-table-column prop="title" label="标题" min-width="260" show-overflow-tooltip />
      <el-table-column label="摘要" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">{{ row.payload?.summary || row.payload?.tag || '—' }}</template>
      </el-table-column>
      <el-table-column prop="sort" label="排序" width="70" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag size="small" :type="row.status === 'published' ? 'success' : 'info'">
            {{ STATUS[row.status] || row.status }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="更新时间" width="150">
        <template #default="{ row }">{{ fmtTime(row.updated_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="190">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link :type="row.status === 'published' ? 'warning' : 'success'" @click="toggleStatus(row)">
            {{ row.status === 'published' ? '下架' : '上架' }}
          </el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" :title="dialogTitle" width="640px">
      <el-form label-width="88px">
        <el-form-item label="类型">
          <el-select v-model="form.kind" style="width: 200px">
            <el-option v-for="(label, k) in KIND" :key="k" :label="label" :value="k" />
          </el-select>
        </el-form-item>
        <el-form-item label="分类">
          <el-select v-model="form.cat" allow-create filterable style="width: 200px">
            <el-option v-for="c in CATS" :key="c" :label="c" :value="c" />
          </el-select>
        </el-form-item>
        <el-form-item label="标题"><el-input v-model="form.title" /></el-form-item>
        <el-form-item label="角标">
          <el-input v-model="form.tag" placeholder="如：案例 / 寄语 / 光荣榜" style="width: 200px" />
        </el-form-item>
        <el-form-item label="摘要">
          <el-input v-model="form.summary" type="textarea" :rows="2" placeholder="列表页展示的一句话" />
        </el-form-item>
        <el-form-item label="正文">
          <el-input v-model="form.body" type="textarea" :rows="8" placeholder="一行一段，空行会被忽略" />
        </el-form-item>
        <el-form-item label="日期">
          <el-input v-model="form.date" placeholder="2026-11-26" style="width: 200px" />
        </el-form-item>
        <el-form-item label="阅读分钟">
          <el-input-number v-model="form.minutes" :min="0" :max="120" />
        </el-form-item>
        <el-form-item label="问 AI 提示">
          <el-input v-model="form.ask" placeholder="详情页「问问 AI」的预置问题" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" :max="999" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="form.status" style="width: 200px">
            <el-option v-for="(label, k) in STATUS" :key="k" :label="label" :value="k" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 12px;
}
.acts {
  display: flex;
  gap: 10px;
  align-items: center;
}
</style>
