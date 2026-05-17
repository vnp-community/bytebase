<!-- i18n: vue-i18n | use t("key") from useI18n() -->
<template>
  <div ref="container" class="h-full" />
</template>

<script lang="ts" setup>
defineOptions({ inheritAttrs: false });

import { onMounted, onUnmounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import type { ReactRoot } from "./bridge-types";

const container = ref<HTMLElement>();
let root: ReactRoot | null = null;
const { locale } = useI18n();
const abortController = new AbortController();

async function render() {
  if (!container.value) return;
  const { mountProjectSidebar, updateProjectSidebarLocale } = await import(
    "./mountProjectSidebar"
  );
  if (abortController.signal.aborted) return;
  if (!root) {
    root = await mountProjectSidebar(container.value, locale.value, abortController.signal);
  } else {
    await updateProjectSidebarLocale(root, locale.value);
  }
}

onMounted(() => render());
watch(locale, () => render());
onUnmounted(() => {
  abortController.abort();
  root?.unmount();
  root = null;
});
</script>
