<template>
  <div :class="containerClasses">
    <!-- Square spinner -->
    <div v-if="type === 'spinner'" class="flex items-center justify-center">
      <div class="relative w-8 h-8">
        <div
          class="absolute inset-0 bg-bg-primary animate-brutal-spin"
        ></div>
      </div>
    </div>

    <!-- Skeleton card -->
    <div v-else-if="type === 'skeleton-card'" :class="skeletonClasses">
      <div class="h-4 bg-bg-tertiary border-2 border-bg-primary w-3/4 mb-4"></div>
      <div class="h-3 bg-bg-tertiary border-2 border-bg-primary w-1/2"></div>
    </div>

    <!-- Skeleton list -->
    <div v-else-if="type === 'skeleton-list'" class="space-y-3">
      <div v-for="n in count" :key="n" class="skeleton-item"></div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  type: {
    type: String,
    default: 'spinner',
    validator: (val) => ['spinner', 'skeleton-card', 'skeleton-list'].includes(val)
  },
  count: {
    type: Number,
    default: 5
  },
  size: {
    type: String,
    default: 'md',
    validator: (val) => ['sm', 'md', 'lg'].includes(val)
  }
})

const containerClasses = computed(() => {
  const classes = ['flex', 'items-center', 'justify-center']

  if (props.size === 'sm') {
    classes.push('w-8', 'h-8')
  } else if (props.size === 'lg') {
    classes.push('w-16', 'h-16')
  } else {
    classes.push('w-12', 'h-12')
  }

  return classes
})

const skeletonClasses = computed(() => {
  const classes = ['border-2', 'border-bg-primary', 'p-6', 'shadow-brutal-sm']

  if (props.size === 'sm') {
    classes.push('text-sm')
  } else if (props.size === 'lg') {
    classes.push('text-lg')
  } else {
    classes.push('text-base')
  }

  return classes
})
</script>

<style scoped>
@keyframes brutal-spin {
  0%, 100% {
    transform: rotate(0deg);
  }
  25% {
    transform: rotate(90deg);
  }
  50% {
    transform: rotate(180deg);
  }
  75% {
    transform: rotate(270deg);
  }
}

.animate-brutal-spin {
  animation: brutal-spin 1.5s steps(4) infinite;
}

.skeleton-item {
  @apply h-16 border border-gray-200 bg-white rounded-lg p-4 shadow-sm;
}

.skeleton-item::before {
  content: '';
  @apply block h-4 bg-gray-100 rounded w-3/4 mb-3;
}

.skeleton-item::after {
  content: '';
  @apply block h-3 bg-gray-100 rounded w-1/2;
}
</style>
