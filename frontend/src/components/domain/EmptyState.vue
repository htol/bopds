<template>
  <div class="flex flex-col items-center justify-center py-16 px-6 text-center">
    <!-- Icon (optional) -->
    <div v-if="icon" class="mb-6 text-6xl font-mono">
      {{ icon }}
    </div>

    <!-- Title -->
    <h2 class="text-4xl font-accent text-bg-primary mb-4 tracking-tight">
      {{ title }}
    </h2>

    <!-- Message -->
    <p class="text-lg font-mono text-text-muted mb-8 max-w-md">
      {{ message }}
    </p>

    <!-- Action Button (optional) -->
    <BaseButton
      v-if="actionLabel && actionCallback"
      variant="accent"
      @click="handleAction"
    >
      {{ actionLabel }}
    </BaseButton>
  </div>
</template>

<script setup>
import BaseButton from '../base/BaseButton.vue'

defineProps({
  icon: {
    type: String,
    default: '∅'
  },
  title: {
    type: String,
    required: true
  },
  message: {
    type: String,
    required: true
  },
  actionLabel: {
    type: String,
    default: undefined
  },
  actionCallback: {
    type: Function,
    default: undefined
  }
})

const handleAction = () => {
  if (actionCallback) {
    actionCallback()
  }
}
</script>
