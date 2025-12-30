<template>
  <div
    :class="cardClasses"
    v-bind="$attrs"
  >
    <slot />
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  variant: {
    type: String,
    default: 'default',
    validator: (val) => ['default', 'inverted'].includes(val)
  },
  hoverable: {
    type: Boolean,
    default: false
  },
  padding: {
    type: String,
    default: 'md',
    validator: (val) => ['none', 'sm', 'md', 'lg'].includes(val)
  }
})

const cardClasses = computed(() => {
  const classes = []

  // Base styles
  classes.push('border-2', 'border-bg-primary', 'transition-all', 'duration-slow')

  // Variant
  if (props.variant === 'inverted') {
    classes.push('bg-bg-primary', 'text-white')
  } else {
    classes.push('bg-bg-secondary', 'text-bg-primary')
  }

  // Shadow
  classes.push('shadow-brutal-sm')

  // Hoverable
  if (props.hoverable) {
    classes.push('hover:shadow-brutal-md', 'hover:-translate-y-px', 'cursor-pointer')
  }

  // Padding
  if (props.padding === 'none') {
    classes.push('p-0')
  } else if (props.padding === 'sm') {
    classes.push('p-4')
  } else if (props.padding === 'lg') {
    classes.push('p-8')
  } else {
    classes.push('p-6')
  }

  return classes
})
</script>
