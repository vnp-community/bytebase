<!-- i18n: vue-i18n | use t("key") from useI18n() -->
<template>
  <div class="app-shell-skeleton">
    <div class="app-shell-skeleton__content">
      <div class="app-shell-skeleton__logo">
        <svg
          width="48"
          height="48"
          viewBox="0 0 48 48"
          fill="none"
          xmlns="http://www.w3.org/2000/svg"
        >
          <rect width="48" height="48" rx="12" fill="#5B5FEF" />
          <path
            d="M14 16h8v4h-8v-4zm0 8h12v4H14v-4zm0 8h16v4H14v-4z"
            fill="white"
            opacity="0.9"
          />
        </svg>
      </div>

      <div class="app-shell-skeleton__progress-wrapper">
        <div class="app-shell-skeleton__progress-bar" />
      </div>

      <div v-if="error" class="app-shell-skeleton__error">
        <p class="app-shell-skeleton__error-title">Failed to start application</p>
        <p class="app-shell-skeleton__error-detail">{{ error }}</p>
        <button class="app-shell-skeleton__retry" @click="$emit('retry')">
          Retry
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  error?: string;
}>();

defineEmits<{
  retry: [];
}>();
</script>

<style scoped>
.app-shell-skeleton {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f9fafb;
  z-index: 9999;
}

.app-shell-skeleton__content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 24px;
}

.app-shell-skeleton__logo {
  animation: pulse 2s ease-in-out infinite;
}

.app-shell-skeleton__progress-wrapper {
  width: 200px;
  height: 3px;
  background: #e5e7eb;
  border-radius: 2px;
  overflow: hidden;
}

.app-shell-skeleton__progress-bar {
  width: 40%;
  height: 100%;
  background: linear-gradient(90deg, #5b5fef, #818cf8);
  border-radius: 2px;
  animation: indeterminate 1.5s ease-in-out infinite;
}

.app-shell-skeleton__error {
  text-align: center;
  max-width: 320px;
}

.app-shell-skeleton__error-title {
  font-size: 16px;
  font-weight: 600;
  color: #dc2626;
  margin-bottom: 8px;
}

.app-shell-skeleton__error-detail {
  font-size: 13px;
  color: #6b7280;
  margin-bottom: 16px;
  word-break: break-word;
}

.app-shell-skeleton__retry {
  padding: 8px 24px;
  background: #5b5fef;
  color: white;
  border: none;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s;
}

.app-shell-skeleton__retry:hover {
  background: #4f46e5;
}

@keyframes pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
}

@keyframes indeterminate {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(350%);
  }
}
</style>
