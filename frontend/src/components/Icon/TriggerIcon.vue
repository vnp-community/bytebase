<!-- i18n: vue-i18n | use t("key") from useI18n() -->
<template>
  <div class="relative">
    <ZapIcon class="text-amber-500" v-bind="$attrs" />
  </div>
</template>
<script setup lang="ts">
import { ZapIcon } from "lucide-vue-next";

defineOptions({
  inheritAttrs: false,
});
</script>
