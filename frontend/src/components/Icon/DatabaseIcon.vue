<!-- i18n: vue-i18n | use t("key") from useI18n() -->
<template>
  <div class="relative">
    <DatabaseIcon v-bind="$attrs" />
  </div>
</template>
<script setup lang="ts">
import { Database as DatabaseIcon } from "lucide-vue-next";
</script>
