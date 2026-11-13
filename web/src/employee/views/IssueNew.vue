<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import { api } from '../../shared/http'

const router = useRouter()
const title = ref('')
const description = ref('')
const severity = ref('normal')
const cateId = ref(null)
const cates = ref([])
const busy = ref(false)

onMounted(async () => {
  try {
    cates.value = await api.get('/api/v1/issue-cates')
  } catch {
    cates.value = []
  }
})

async function submit() {
  if (!title.value.trim()) {
    showToast('请填写问题标题')
    return
  }
  busy.value = true
  try {
    const issue = await api.post('/api/v1/issues', {
      title: title.value.trim(),
      description: description.value.trim(),
      severity: severity.value,
      cate_id: cateId.value || null,
    })
    showToast('已上报')
    router.replace(`/issues/${issue.id}`)
  } catch (e) {
    showToast(e.message)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="app-shell">
    <van-nav-bar title="上报问题" left-arrow @click-left="$router.back()" />
    <div class="page pad">
      <van-cell-group inset>
        <van-field v-model="title" label="标题" placeholder="一句话说明问题，如：1302 端头支护不足" maxlength="60" />
        <van-field
          v-model="description"
          label="描述"
          type="textarea"
          rows="4"
          autosize
          placeholder="位置、现象、风险，便于区队受理整改"
          maxlength="500"
        />
        <van-field label="分类">
          <template #input>
            <van-radio-group v-model="cateId" direction="horizontal" style="flex-wrap: wrap">
              <van-radio v-for="c in cates" :key="c.id" :name="c.id">{{ c.name }}</van-radio>
            </van-radio-group>
          </template>
        </van-field>
        <van-field label="严重程度">
          <template #input>
            <van-radio-group v-model="severity" direction="horizontal">
              <van-radio name="low">一般</van-radio>
              <van-radio name="normal">较重</van-radio>
              <van-radio name="high">重大</van-radio>
            </van-radio-group>
          </template>
        </van-field>
      </van-cell-group>

      <div style="margin: 16px 0">
        <van-button block type="primary" :loading="busy" @click="submit">提交上报</van-button>
      </div>
      <p class="muted" style="padding: 0 8px">上报后可跟踪受理、整改、验收与闭环全过程。</p>
    </div>
  </div>
</template>
