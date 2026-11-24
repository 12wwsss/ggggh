<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../../shared/http'
import { fmtSize, fmtTime } from '../../shared/format'

const rows = ref([])
const loading = ref(false)
const dialog = ref(false)
const saving = ref(false)
const file = ref(null)
const form = ref({ title: '', category: '操作视频', url: '' })
const CATS = ['操作视频', '设备原理', '规程制度', '事故案例']

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/v1/media')
    rows.value = data.list || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function open() {
  form.value = { title: '', category: '操作视频', url: '' }
  file.value = null
  dialog.value = true
}

function onFile(f) {
  file.value = f.raw
  if (!form.value.title) form.value.title = f.name.replace(/\.[^.]+$/, '')
}

async function submit() {
  if (!file.value && !form.value.url) {
    return ElMessage.warning('请选择视频文件或填写外链地址')
  }
  saving.value = true
  try {
    if (file.value) {
      await api.upload('/api/v1/media/upload', file.value, {
        title: form.value.title || file.value.name,
        category: form.value.category,
      })
    } else {
      await api.post('/api/v1/media/link', {
        title: form.value.title,
        category: form.value.category,
        url: form.value.url,
      })
    }
    ElMessage.success('已添加')
    dialog.value = false
    load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  try {
    await ElMessageBox.confirm(`删除「${row.title}」？`, '确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await api.del(`/api/v1/media/${row.id}`)
    ElMessage.success('已删除')
    load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="toolbar">
      <div class="muted">视频上传后员工在小程序「视频资料」里直接播放；也可以填外链（矿上文件服务器/OSS）。</div>
      <el-button type="primary" @click="open">上传视频</el-button>
    </div>

    <el-table :data="rows" v-loading="loading" size="small">
      <el-table-column prop="title" label="标题" min-width="220" show-overflow-tooltip />
      <el-table-column prop="category" label="分类" width="120" />
      <el-table-column label="大小" width="100">
        <template #default="{ row }">{{ row.size_bytes ? fmtSize(row.size_bytes) : '外链' }}</template>
      </el-table-column>
      <el-table-column label="地址" min-width="240" show-overflow-tooltip>
        <template #default="{ row }">{{ row.url }}</template>
      </el-table-column>
      <el-table-column label="时间" width="120">
        <template #default="{ row }">{{ row.created_at }}</template>
      </el-table-column>
      <el-table-column label="操作" width="90">
        <template #default="{ row }">
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" title="上传视频" width="560px">
      <el-form label-width="90px">
        <el-form-item label="标题">
          <el-input v-model="form.title" placeholder="不填则用文件名" />
        </el-form-item>
        <el-form-item label="分类">
          <el-select v-model="form.category" filterable allow-create default-first-option style="width: 100%">
            <el-option v-for="c in CATS" :key="c" :label="c" :value="c" />
          </el-select>
        </el-form-item>
        <el-form-item label="视频文件">
          <el-upload :auto-upload="false" :limit="1" accept="video/*" :on-change="onFile">
            <el-button>选择文件</el-button>
            <template #tip><div class="muted">支持 mp4/webm/mov/ogg，单文件不超过 300MB</div></template>
          </el-upload>
        </el-form-item>
        <el-form-item label="或外链">
          <el-input v-model="form.url" placeholder="https://…（可不传文件，直接填地址）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
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
</style>
