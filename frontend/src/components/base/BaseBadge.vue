<template>
  <span :class="badgeClasses">
    <slot />
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  variant: {
    type: String,
    default: 'outline',
    validator: (val) => ['accent', 'black', 'outline'].includes(val)
  },
  size: {
    type: String,
    default: 'md',
    validator: (val) => ['sm', 'md'].includes(val)
  }
})

const badgeClasses = computed(() => {
  const classes = []

  // Base styles
  classes.push('inline-flex', 'items-center', 'justify-center', 'font-medium', 'border', 'rounded')

  // Size
  if (props.size === 'sm') {
    classes.push('px-2', 'py-0.5', 'text-xs')
  } else {
    classes.push('px-2.5', 'py-1', 'text-sm')
  }

  // Variant
  if (props.variant === 'accent') {
    classes.push('bg-accent-primary', 'border-accent-primary', 'text-white')
  } else if (props.variant === 'black') {
    classes.push('bg-gray-900', 'border-gray-900', 'text-white')
  } else {
    classes.push('bg-transparent', 'border-gray-300', 'text-gray-700')
  }

  return classes
})
</script>
