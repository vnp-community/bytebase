<!-- i18n: vue-i18n | use t("key") from useI18n() -->
<template>
  <div class="relative">
    <LinkIcon class="w-4 h-4" v-bind="$attrs" />
  </div>
</template>

<script setup lang="ts">
import { LinkIcon } from "lucide-vue-next";
</script>
