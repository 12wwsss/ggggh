<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../../shared/http'
import { docStatus, fmtSize, fmtTime } from '../../shared/format'
import SealStamp from '../components/SealStamp.vue'

const router = useRouter()
const rows = ref([])
const total = ref(0)
const loading = ref(false)
const sealedId = ref(0)
const query = reactive({ page: 1, page_size: 20, status: '', keyword: '' })

const uploadVisible = ref(false)
const uploading = ref(false)
const uploadForm = reactive({ title: '', category: '规程', file: null, force: false })

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/v1/docs', { ...query })
    rows.value = data.list || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)

function onFileChange(file) {
  uploadForm.file = file.raw
  if (!uploadForm.title) uploadForm.title = file.name.replace(/\.[^.]+$/, '')
}

async function submitUpload() {
  if (!uploadForm.file) {
    ElMessage.warning('请选择文件')
    return
  }
  uploading.value = true
  try {
    const res = await api.upload('/api/v1/docs', uploadForm.file, {
      title: uploadForm.title,
      category: uploadForm.category,
      force: String(uploadForm.force),
    })
    const doc = res?.doc || res
    uploadVisible.value = false
    uploadForm.file = null
    uploadForm.title = ''
    if (res?.duplicate) {
      ElMessage.warning(`已存在相同内容的文档（#${doc.id}），直接复用，未重复入库`)
      router.push(`/docs/${doc.id}`)
      return
    }
    ElMessage.success('已上传，正在后台解析')
    query.page = 1
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    uploading.value = false
  }
}

async function publish(row) {
  try {
    await ElMessageBox.confirm(`发布《${row.title}》后，条款将进入检索库。`, '确认发布', { type: 'warning' })
    await api.post(`/api/v1/docs/${row.id}/publish`)
    ElMessage.success('已发布，正在生成向量')
    sealedId.value = row.id
    await load()
    setTimeout(() => {
      if (sealedId.value === row.id) sealedId.value = 0
    }, 1600)
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(e.message)
  }
}

async function reject(row) {
  try {
    const { value } = await ElMessageBox.prompt('驳回原因（会记录到文档）', '驳回文档', {
      inputPlaceholder: '例如：条款切分错误，需重新解析',
    })
    await api.post(`/api/v1/docs/${row.id}/reject`, { reason: value || '' })
    ElMessage.success('已驳回')
    await load()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(e.message)
  }
}

async function reparse(row) {
  try {
    await api.post(`/api/v1/docs/${row.id}/reparse`)
    ElMessage.success('已重新投递解析任务')
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function remove(row) {
  try {
    await ElMessageBox.confirm(`删除《${row.title}》会同时删除切片与向量。`, '确认删除', { type: 'error' })
    await api.del(`/api/v1/docs/${row.id}`)
    ElMessage.success('已删除')
    await load()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(e.message)
  }
}
</script>

<template>
  <div class="panel">
    <div style="display: flex; gap: 10px; margin-bottom: 14px; flex-wrap: wrap">
      <el-input v-model="query.keyword" placeholder="按标题搜索" clearable style="width: 220px" @change="load" />
      <el-select v-model="query.status" placeholder="全部状态" clearable style="width: 140px" @change="load">
        <el-option label="解析中" value="parsing" />
        <el-option label="待审核" value="pending_review" />
        <el-option label="已发布" value="published" />
        <el-option label="已驳回" value="rejected" />
        <el-option label="解析失败" value="failed" />
      </el-select>
      <div style="flex: 1"></div>
      <el-button type="primary" @click="uploadVisible = true">上传文档</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" size="small">
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column prop="title" label="标题" min-width="220" show-overflow-tooltip />
      <el-table-column prop="category" label="分类" width="100" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <SealStamp
            v-if="row.status === 'published'"
            text="已发布"
            :animate="sealedId === row.id"
          />
          <el-tag v-else :type="docStatus[row.status]?.type || 'info'" size="small">
            {{ docStatus[row.status]?.text || row.status }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="chunk_count" label="切片" width="72" />
      <el-table-column label="大小" width="90">
        <template #default="{ row }">{{ fmtSize(row.file_size) }}</template>
      </el-table-column>
      <el-table-column prop="uploader" label="上传人" width="90" />
      <el-table-column label="时间" width="150">
        <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="250">
        <template #default="{ row }">
          <el-button link type="primary" @click="router.push(`/docs/${row.id}`)">切片</el-button>
          <el-button v-if="row.status === 'pending_review'" link type="success" @click="publish(row)">发布</el-button>
          <el-button v-if="row.status === 'pending_review'" link type="warning" @click="reject(row)">驳回</el-button>
          <el-button v-if="row.status === 'failed'" link type="warning" @click="reparse(row)">重解析</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div style="margin-top: 14px; display: flex; justify-content: flex-end">
      <el-pagination
        v-model:current-page="query.page"
        v-model:page-size="query.page_size"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="load"
      />
    </div>

    <el-dialog v-model="uploadVisible" title="上传知识文档" width="480px">
      <el-form label-width="72px">
        <el-form-item label="文件">
          <el-upload
            :auto-upload="false"
            :limit="1"
            :on-change="onFileChange"
            accept=".docx,.doc,.pdf,.txt,.md,.csv,.json"
          >
            <el-button>选择文件</el-button>
            <template #tip>
              <div class="muted">
                规程按「第X条」切分；制度、操作规程、事故案例没有条款号时按段落切分；FAQ 用 CSV（问题,答案）或 JSON。
                doc / pdf 需要服务器安装 LibreOffice。
              </div>
            </template>
          </el-upload>
        </el-form-item>
        <el-form-item label="标题">
          <el-input v-model="uploadForm.title" placeholder="默认取文件名" />
        </el-form-item>
        <el-form-item label="分类">
          <el-input v-model="uploadForm.category" placeholder="规程 / 制度 / 操作规程 / 事故案例 / FAQ" />
        </el-form-item>
        <el-form-item label="去重">
          <el-switch v-model="uploadForm.force" active-text="强制重新入库" inactive-text="相同内容复用已有文档" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="uploadVisible = false">取消</el-button>
        <el-button type="primary" :loading="uploading" @click="submitUpload">上传</el-button>
      </template>
    </el-dialog>
  </div>
</template>
