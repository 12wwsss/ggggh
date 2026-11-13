<script setup>
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api, fetchText } from '../../shared/http'
import { docStatus, fmtSize, fmtTime } from '../../shared/format'

const route = useRoute()
const router = useRouter()
const doc = ref(null)
const chunks = ref([])
const loading = ref(false)

const versions = ref([])
const versionsDrawer = ref(false)
const diff = ref(null)
const diffTitle = ref('')

async function load() {
  loading.value = true
  try {
    doc.value = await api.get(`/api/v1/docs/${route.params.id}`)
    const data = await api.get(`/api/v1/docs/${route.params.id}/chunks`, undefined)
    chunks.value = data.list || data || []
    versions.value = await api.get(`/api/v1/docs/${route.params.id}/versions`)
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function compare(v) {
  try {
    const to = doc.value?.version || 0
    diff.value = await api.get(`/api/v1/docs/${route.params.id}/diff`, { from: v.version, to })
    diffTitle.value = `v${v.version} → v${to}`
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function rollback(v) {
  try {
    await ElMessageBox.confirm(
      `回滚到 v${v.version} 会生成新版本 v${(doc.value?.version || 0) + 1}，并重建检索向量。`,
      '确认回滚',
      { type: 'warning' },
    )
    const res = await api.post(`/api/v1/docs/${route.params.id}/versions/${v.version}/rollback`)
    ElMessage.success(`已回滚，当前版本 v${res.version}`)
    await load()
    versionsDrawer.value = true
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(e.message)
  }
}

async function publish() {
  try {
    await api.post(`/api/v1/docs/${route.params.id}/publish`)
    ElMessage.success('已发布')
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function reject() {
  try {
    const { value } = await ElMessageBox.prompt('驳回原因', '驳回')
    await api.post(`/api/v1/docs/${route.params.id}/reject`, { reason: value || '' })
    ElMessage.success('已驳回')
    await load()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error(e.message)
  }
}

async function copyText(row) {
  try {
    const text = await fetchText(`/api/v1/chunks/${row.id}/text`)
    await navigator.clipboard.writeText(text)
    ElMessage.success('原文已复制')
  } catch (e) {
    ElMessage.error('复制失败：' + e.message)
  }
}
</script>

<template>
  <div v-loading="loading">
    <div class="panel" style="margin-bottom: 16px">
      <div style="display: flex; justify-content: space-between; align-items: flex-start">
        <div>
          <h3 class="panel-title" style="margin-bottom: 8px">{{ doc?.title }}</h3>
          <el-descriptions :column="4" size="small">
            <el-descriptions-item label="状态">
              <el-tag :type="docStatus[doc?.status]?.type || 'info'" size="small">
                {{ docStatus[doc?.status]?.text || doc?.status }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="分类">{{ doc?.category }}</el-descriptions-item>
            <el-descriptions-item label="切片数">{{ doc?.chunk_count }}</el-descriptions-item>
            <el-descriptions-item label="大小">{{ fmtSize(doc?.file_size) }}</el-descriptions-item>
            <el-descriptions-item label="上传人">{{ doc?.uploader || '—' }}</el-descriptions-item>
            <el-descriptions-item label="上传时间">{{ fmtTime(doc?.created_at) }}</el-descriptions-item>
            <el-descriptions-item label="发布时间">{{ fmtTime(doc?.published_at) }}</el-descriptions-item>
            <el-descriptions-item label="版本">v{{ doc?.version }}</el-descriptions-item>
          </el-descriptions>
          <div v-if="doc?.error_msg" style="color: #d93025; margin-top: 8px">{{ doc.error_msg }}</div>
        </div>
        <div style="display: flex; gap: 8px">
          <el-button @click="versionsDrawer = true">版本（{{ versions.length }}）</el-button>
          <el-button v-if="doc?.status === 'pending_review'" type="success" @click="publish">发布</el-button>
          <el-button v-if="doc?.status === 'pending_review'" type="warning" @click="reject">驳回</el-button>
          <el-button @click="router.back()">返回</el-button>
        </div>
      </div>
    </div>

    <el-drawer v-model="versionsDrawer" title="版本历史" size="620px">
      <el-table :data="versions" size="small">
        <el-table-column label="版本" width="80">
          <template #default="{ row }">v{{ row.version }}</template>
        </el-table-column>
        <el-table-column prop="chunk_count" label="切片" width="70" />
        <el-table-column prop="operator" label="发布人" width="100" />
        <el-table-column label="时间" width="150">
          <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150">
          <template #default="{ row }">
            <el-button link type="primary" @click="compare(row)">对比当前</el-button>
            <el-button
              link
              type="danger"
              :disabled="row.version === doc?.version"
              @click="rollback(row)"
            >
              回滚
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <template v-if="diff">
        <el-divider content-position="left">{{ diffTitle }} 条款差异</el-divider>
        <div style="display: flex; gap: 16px; flex-wrap: wrap; font-size: 13px">
          <span>新增 {{ diff.added.length }}</span>
          <span>删除 {{ diff.removed.length }}</span>
          <span>修改 {{ diff.changed.length }}</span>
          <span class="muted">未变 {{ diff.same }}</span>
        </div>
        <div v-for="(group, key) in { added: '新增', removed: '删除', changed: '修改' }" :key="key" style="margin-top: 12px">
          <div v-if="diff[key].length" class="muted">{{ group }}</div>
          <span v-for="c in diff[key]" :key="key + c" class="clause-chip" style="margin: 6px 6px 0 0">{{ c }}</span>
        </div>
      </template>
    </el-drawer>

    <div class="panel">
      <h3 class="panel-title">条款切片（{{ chunks.length }}）</h3>
      <el-table :data="chunks" size="small" max-height="620">
        <el-table-column prop="seq" label="#" width="56" />
        <el-table-column label="条款号" width="130">
          <template #default="{ row }">
            <span style="font-weight: 600">{{ row.clause_no || '（无条款号）' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="chapter" label="章/节" width="200" show-overflow-tooltip />
        <el-table-column prop="content" label="正文" min-width="420" show-overflow-tooltip />
        <el-table-column prop="char_count" label="字数" width="72" />
        <el-table-column label="发布" width="72">
          <template #default="{ row }">
            <el-tag :type="row.published ? 'success' : 'info'" size="small">{{ row.published ? '是' : '否' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="90">
          <template #default="{ row }">
            <el-button link type="primary" @click="copyText(row)">复制原文</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>
