<template>
  <button
    :type="type"
    :disabled="disabled || loading"
    :class="buttonClasses"
    v-bind="$attrs"
    @click="handleClick"
  >
    <span v-if="loading" class="inline-block w-4 h-4 mr-2 animate-spin">◧</span>
    <component v-else-if="iconLeft" :is="iconLeft" class="w-4 h-4" />
    <slot />
    <component v-if="iconRight && !loading" :is="iconRight" class="w-4 h-4" />
  </button>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  variant: {
    type: String,
    default: 'primary',
    validator: (val) => ['primary', 'secondary', 'accent', 'ghost'].includes(val)
  },
  size: {
    type: String,
    default: 'md',
    validator: (val) => ['sm', 'md', 'lg'].includes(val)
  },
  disabled: Boolean,
  loading: Boolean,
  iconLeft: Object,
  iconRight: Object,
  type: {
    type: String,
    default: 'button'
  }
})

const emit = defineEmits(['click'])

const handleClick = (e) => {
  if (!props.disabled && !props.loading) {
    emit('click', e)
  }
}

const buttonClasses = computed(() => {
  const classes = []

  // Base styles
  classes.push('inline-flex', 'items-center', 'justify-center', 'gap-2', 'font-medium')
  classes.push('transition-all', 'duration-200')
  classes.push('disabled:opacity-50', 'disabled:cursor-not-allowed')
  classes.push('rounded')

  // Size variants
  if (props.size === 'sm') {
    classes.push('px-3', 'py-1.5', 'text-sm')
  } else if (props.size === 'lg') {
    classes.push('px-6', 'py-3', 'text-base')
  } else {
    classes.push('px-4', 'py-2', 'text-sm')
  }

  // Color variants
  if (props.variant === 'primary') {
    classes.push(
      'border', 'border-gray-300', 'bg-white', 'text-gray-700',
      'shadow-sm',
      'hover:bg-gray-50', 'hover:border-gray-400',
      'active:translate-y-0'
    )
  } else if (props.variant === 'accent') {
    classes.push(
      'border', 'border-accent-primary', 'bg-accent-primary', 'text-white',
      'shadow-sm',
      'hover:bg-accent-hover',
      'active:translate-y-0'
    )
  } else if (props.variant === 'secondary') {
    classes.push(
      'border', 'border-gray-200', 'bg-gray-50', 'text-gray-600',
      'shadow-sm',
      'hover:bg-white', 'hover:border-accent-primary', 'hover:text-accent-primary', 'hover:shadow-md',
      'active:translate-y-0'
    )
  } else if (props.variant === 'ghost') {
    classes.push(
      'border', 'border-gray-300', 'bg-transparent', 'text-gray-700',
      'hover:bg-gray-50',
      'active:translate-y-0'
    )
  }

  return classes
})
</script>
