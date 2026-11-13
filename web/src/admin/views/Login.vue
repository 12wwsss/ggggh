<script setup>
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { clearSession, login, getUser } from '../../shared/http'

const router = useRouter()
const route = useRoute()
const form = ref({ username: '', password: '' })
const loading = ref(false)

async function submit() {
  if (!form.value.username || !form.value.password) {
    ElMessage.warning('请输入账号与口令')
    return
  }
  loading.value = true
  try {
    await login(form.value.username.trim(), form.value.password)
    const user = getUser()
    if (user?.role === 'employee') {
      clearSession()
      ElMessage.error('员工账号请使用移动端入口 /')
      return
    }
    router.replace(route.query.redirect || '/dashboard')
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-wrap">
    <div class="login-card admin-login">
      <h1>煤矿安全员AI智能体平台</h1>
      <p>管理后台 · 文档、题库、宣贯与问题闭环</p>
      <el-form label-position="top" @submit.prevent="submit">
        <el-form-item label="账号">
          <el-input v-model="form.username" placeholder="admin" size="large" autocomplete="username" />
        </el-form-item>
        <el-form-item label="口令">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="请输入口令"
            size="large"
            show-password
            autocomplete="current-password"
            @keyup.enter="submit"
          />
        </el-form-item>
        <el-button type="primary" size="large" style="width: 100%" :loading="loading" @click="submit">
          登录
        </el-button>
      </el-form>
      <p style="margin-top: 14px">演示账号 admin / Admin@123456</p>
    </div>
  </div>
</template>
