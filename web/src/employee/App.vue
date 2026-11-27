<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const showTab = computed(() => Boolean(route.meta.tab))
</script>

<template>
  <div class="app-shell">
    <div class="app-view">
      <router-view v-slot="{ Component }">
        <component :is="Component" />
      </router-view>
    </div>

    <van-tabbar v-if="showTab" route active-color="#a5311f" inactive-color="#5b5347" placeholder>
      <van-tabbar-item replace to="/" icon="home-o">首页</van-tabbar-item>
      <van-tabbar-item replace to="/ask" icon="chat-o">问答</van-tabbar-item>
      <van-tabbar-item replace to="/learn" icon="bookmark-o">学习</van-tabbar-item>
      <van-tabbar-item replace to="/me" icon="user-o">我的</van-tabbar-item>
    </van-tabbar>
  </div>
</template>

<style scoped>
.app-view {
  flex: 1;
  min-height: 0;
  display: flex;
}
.app-view > * {
  flex: 1;
  min-height: 0;
}
</style>
