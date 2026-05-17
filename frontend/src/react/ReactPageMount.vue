<!-- i18n: vue-i18n | use t("key") from useI18n() -->
<template>
  <div ref="container" :class="containerClass" />
</template>

<script lang="ts" setup>
defineOptions({ inheritAttrs: false });

import { computed, onMounted, onUnmounted, ref, useAttrs, watch } from "vue";
import { useI18n } from "vue-i18n";
import { bridgeManager } from "./BridgeLifecycleManager";
import { useNotificationStore } from "@/store";

// `containerClass` defaults to `h-full` so full-height pages (Welcome,
// HistoryPane, AccessPane, etc.) keep the previous behavior. Callers that
// mount a natural-height surface (e.g. a toolbar inside a flex-col) should
// override with something like `w-full` so the wrapper doesn't stretch to
// fill its flex-column parent.
const props = withDefaults(
  defineProps<{
    page: string;
    pageProps?: Record<string, unknown>;
    containerClass?: string;
  }>(),
  { containerClass: "h-full" }
);
const containerClass = computed(() => props.containerClass);

const attrs = useAttrs();
const { locale } = useI18n();
const container = ref<HTMLElement>();
const abortController = new AbortController();

const pageProps = computed(() => {
  const a = attrs as Record<string, unknown>;
  if (props.pageProps || Object.keys(a).length > 0) {
    return {
      ...props.pageProps,
      ...a,
    };
  }
  return undefined;
});

async function render() {
  if (!container.value) return;

  // Sync locale before mount
  try {
    const i18nModule = await import("./i18n");
    if (i18nModule.default.language !== locale.value) {
      await i18nModule.default.changeLanguage(locale.value);
    }
  } catch {
    // i18n sync is best-effort
  }

  try {
    await bridgeManager.mount(
      container.value,
      { pageName: props.page, props: pageProps.value },
      abortController.signal
    );
  } catch (error) {
    if (abortController.signal.aborted) return;
    useNotificationStore().pushNotification({
      module: "bytebase",
      style: "CRITICAL",
      title: `Failed to load page: ${props.page}`,
      description: String(error),
    });
  }
}

onMounted(() => render());
watch(locale, () => render());
watch([() => props.page, pageProps], () => render());
onUnmounted(() => {
  abortController.abort();
  bridgeManager.unmountCurrent();
});
</script>
