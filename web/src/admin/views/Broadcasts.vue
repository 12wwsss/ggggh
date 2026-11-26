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
const posts = ref([])
const dialog = ref(false)
const form = reactive({
  title: '',
  source: '',
  content: '',
  dept_ids: [],
  post_ids: [],
  require_ack: true,
  deadline: null,
  publish: true,
})
const drafting = ref(false)
const draftPoints = ref([])
const draftNotice = ref('')

const recordsDialog = ref(false)
const records = ref([])
const current = ref(null)

const quizDialog = ref(false)
const quizTitle = ref('')
const quiz = ref({ attempts: 0, users: 0, avg_pct: 0, pass_rate: 0, rows: [] })

async function openQuiz(row) {
  quizTitle.value = row.title
  try {
    const data = await api.get(`/api/v1/broadcast/tasks/${row.id}/quiz-stats`)
    quiz.value = data
    quizDialog.value = true
  } catch (e) {
    ElMessage.error(e.message)
  }
}

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
  try {
    const data = await api.get('/api/v1/posts', { all: 1 })
    posts.value = (data.list || []).filter((p) => p.status === 'active').map((p) => p.name)
  } catch {
    posts.value = []
  }
})

function openCreate() {
  Object.assign(form, {
    title: '',
    source: '',
    content: '',
    dept_ids: [],
    post_ids: [],
    require_ack: true,
    deadline: null,
    publish: true,
  })
  draftPoints.value = []
  draftNotice.value = ''
  dialog.value = true
}

// 建设方案六（6）：新制度发布后平台自动生成解读要点，管理员确认后再发布。
async function genDraft() {
  const body = (form.source || form.content).trim()
  if (!body) {
    ElMessage.warning('先把制度原文粘到「制度原文」，或在内容里写清要点')
    return
  }
  drafting.value = true
  try {
    const d = await api.post('/api/v1/broadcast/draft', { title: form.title, content: body })
    draftPoints.value = d.points || []
    draftNotice.value = d.notice || (d.source === 'llm' ? `由 ${d.model || '模型'} 生成初稿，请核对` : '')
    if (d.content) form.content = d.content
    ElMessage.success('已生成解读要点初稿，请核对后发布')
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    drafting.value = false
  }
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
      post_ids: form.post_ids,
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
      <el-table-column label="范围" min-width="150">
        <template #default="{ row }">
          <span v-if="!row.dept_ids?.length && !row.post_ids?.length" class="muted">全员</span>
          <span v-else class="muted">
            {{ row.dept_ids?.length ? row.dept_ids.length + ' 个区队' : '' }}
            {{ row.dept_ids?.length && row.post_ids?.length ? ' · ' : '' }}
            {{ row.post_ids?.length ? row.post_ids.join('、') : '' }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="截止" width="150">
        <template #default="{ row }">{{ row.deadline ? fmtTime(row.deadline) : '—' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="240">
        <template #default="{ row }">
          <el-button v-if="row.status === 'draft'" link type="success" @click="publish(row)">发布</el-button>
          <el-button link type="primary" @click="openRecords(row)">签收明细</el-button>
          <el-button link type="warning" @click="openQuiz(row)">答题统计</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" title="新建宣贯任务" width="620px">
      <el-form label-width="98px">
        <el-form-item label="标题"><el-input v-model="form.title" /></el-form-item>
        <el-form-item label="制度原文">
          <el-input
            v-model="form.source"
            type="textarea"
            :rows="4"
            placeholder="粘贴新制度/新规程原文，用于自动生成解读要点（不发布原文）"
          />
        </el-form-item>
        <el-form-item label="解读要点">
          <div style="width: 100%">
            <div style="display: flex; gap: 10px; align-items: center; margin-bottom: 8px">
              <el-button size="small" :loading="drafting" @click="genDraft">AI 生成要点</el-button>
              <span class="muted">{{ draftNotice }}</span>
            </div>
            <ol v-if="draftPoints.length" class="draft-points">
              <li v-for="(p, i) in draftPoints" :key="i">{{ p }}</li>
            </ol>
          </div>
        </el-form-item>
        <el-form-item label="发布正文">
          <el-input v-model="form.content" type="textarea" :rows="5" placeholder="员工看到的宣贯内容" />
        </el-form-item>
        <el-form-item label="范围">
          <el-select v-model="form.dept_ids" multiple placeholder="区队，不选表示不限" style="width: 100%">
            <el-option v-for="d in depts" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="岗位">
          <el-select v-model="form.post_ids" multiple allow-create filterable placeholder="岗位，不选表示不限岗位" style="width: 100%">
            <el-option v-for="p in posts" :key="p" :label="p" :value="p" />
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

    <el-dialog v-model="quizDialog" :title="`答题统计 · ${quizTitle}`" width="600px">
      <div class="quiz-sum">
        <span>参与人数 <strong>{{ quiz.users }}</strong></span>
        <span>答题次数 <strong>{{ quiz.attempts }}</strong></span>
        <span>平均分 <strong>{{ quiz.avg_pct }}</strong></span>
        <span>及格率 <strong>{{ quiz.pass_rate }}%</strong></span>
      </div>
      <el-table :data="quiz.rows" size="small" max-height="380">
        <el-table-column prop="name" label="姓名" width="100" />
        <el-table-column prop="attempts" label="次数" width="80" />
        <el-table-column prop="best_pct" label="最好成绩" width="100" />
        <el-table-column prop="last_at" label="最近答题" width="140" />
      </el-table>
      <el-empty v-if="!quiz.rows.length" description="还没有人答题" :image-size="60" />
      <template #footer>
        <el-button @click="quizDialog = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.quiz-sum {
  display: flex;
  gap: 24px;
  color: #59636e;
  margin-bottom: 12px;
}
.quiz-sum strong {
  color: #a5311f;
  font-size: 18px;
  margin-left: 4px;
}
.draft-points {
  margin: 0;
  padding-left: 20px;
  line-height: 1.9;
  color: #2f3438;
}
</style>
