<!-- i18n: vue-i18n | use t("key") from useI18n() -->
<template>
  <div class="relative">
    <SchemaIcon v-bind="$attrs" />
  </div>
</template>
<script setup lang="ts">
import { BoxIcon as SchemaIcon } from "lucide-vue-next";
</script>
