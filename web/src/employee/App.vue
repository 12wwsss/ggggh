<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const showTab = computed(() => Boolean(route.meta.tab))
</script>

<template>
  <div class="app-shell">
    <router-view v-slot="{ Component }">
      <component :is="Component" />
    </router-view>

    <van-tabbar v-if="showTab" route active-color="#a5311f" inactive-color="#8a8172" placeholder>
      <van-tabbar-item replace to="/" icon="home-o">首页</van-tabbar-item>
      <van-tabbar-item replace to="/ask" icon="chat-o">问规程</van-tabbar-item>
      <van-tabbar-item replace to="/me" icon="user-o">我的</van-tabbar-item>
    </van-tabbar>
  </div>
</template>
