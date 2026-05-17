<!-- i18n: vue-i18n | use t("key") from useI18n() -->
<template>
  <div
    class="text-md font-normal"
    :class="[dark ? 'text-matrix-green-hover' : 'text-control-light']"
  >
    {{ $t("sql-editor.no-data-available") }}
  </div>
</template>

<script lang="ts" setup>
defineProps<{
  dark: boolean;
}>();
</script>
