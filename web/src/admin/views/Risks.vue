<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '../../shared/http'

const rows = ref([])
const loading = ref(false)
const dialog = ref(false)
const saving = ref(false)
const form = ref({ post: '通用', category: '', title: '', description: '', level: '中', measuresText: '', clausesText: '', caseText: '' })
const POSTS = ['通用', '采煤', '掘进', '通风', '机电', '运输']

async function load() {
  loading.value = true
  try {
    const data = await api.get('/api/v1/risks')
    rows.value = data.list || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function open() {
  form.value = { post: '通用', category: '', title: '', description: '', level: '中', measuresText: '', clausesText: '', caseText: '' }
  dialog.value = true
}

async function submit() {
  if (!form.value.title) return ElMessage.warning('请填写风险点名称')
  saving.value = true
  try {
    await api.post('/api/v1/risks', {
      post: form.value.post,
      category: form.value.category,
      title: form.value.title,
      description: form.value.description,
      level: form.value.level,
      measures: form.value.measuresText.split('\n').map((s) => s.trim()).filter(Boolean),
      clause_refs: form.value.clausesText.split(/[，,、\s]+/).map((s) => s.trim()).filter(Boolean),
      case_text: form.value.caseText,
    })
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
    await api.del(`/api/v1/risks/${row.id}`)
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
      <div class="muted">岗位风险点会显示在小程序「学习 → 岗位风险」，员工可按岗位查看风险与管控措施。</div>
      <el-button type="primary" @click="open">新增风险点</el-button>
    </div>

    <el-table :data="rows" v-loading="loading" size="small">
      <el-table-column prop="post" label="岗位" width="90" />
      <el-table-column prop="category" label="分类" width="120" />
      <el-table-column prop="title" label="风险点" min-width="220" show-overflow-tooltip />
      <el-table-column prop="level" label="等级" width="80" />
      <el-table-column prop="description" label="风险描述" min-width="260" show-overflow-tooltip />
      <el-table-column label="措施" width="80">
        <template #default="{ row }">{{ (row.measures || []).length }} 条</template>
      </el-table-column>
      <el-table-column label="案例" width="70">
        <template #default="{ row }">
          <el-tag v-if="row.case_text" type="success" size="small">有</el-tag>
          <span v-else class="muted">无</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="90">
        <template #default="{ row }">
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" title="新增风险点" width="600px">
      <el-form label-width="90px">
        <el-form-item label="岗位">
          <el-select v-model="form.post" style="width: 100%">
            <el-option v-for="p in POSTS" :key="p" :label="p" :value="p" />
          </el-select>
        </el-form-item>
        <el-form-item label="分类">
          <el-input v-model="form.category" placeholder="如：顶板管理" />
        </el-form-item>
        <el-form-item label="风险点">
          <el-input v-model="form.title" placeholder="如：空顶作业" />
        </el-form-item>
        <el-form-item label="等级">
          <el-select v-model="form.level" style="width: 160px">
            <el-option v-for="l in ['低', '中', '高', '重大']" :key="l" :label="l" :value="l" />
          </el-select>
        </el-form-item>
        <el-form-item label="风险描述">
          <el-input v-model="form.description" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="管控措施">
          <el-input v-model="form.measuresText" type="textarea" :rows="4" placeholder="一行一条" />
        </el-form-item>
        <el-form-item label="典型案例">
          <el-input
            v-model="form.caseText"
            type="textarea"
            :rows="4"
            placeholder="选填。写清经过与教训，例如：某矿 2023 年因未敲帮问顶导致片帮伤人……"
          />
        </el-form-item>
        <el-form-item label="依据条款">
          <el-input v-model="form.clausesText" placeholder="如：第四十八条、第五十二条" />
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
