<script setup>
// 盖章：终态（已发布 / 已闭环）用一枚朱红印章表达，而不是又一个彩色标签。
// animate 只在状态刚变化时打开，避免整页同时盖一堆章。
defineProps({
  text: { type: String, required: true },
  animate: { type: Boolean, default: false },
})
</script>

<template>
  <span class="seal-stamp" :class="{ 'is-press': animate }">{{ text }}</span>
</template>
