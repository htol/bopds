<template>
  <article
    :class="cardClasses"
    @click="handleClick"
  >
    <div class="flex justify-between items-center gap-4">
      <!-- Author Name -->
      <div class="flex-1 min-w-0">
        <h3 class="text-lg font-display font-semibold text-gray-900 truncate group-hover:text-accent-primary transition-colors duration-200">
          {{ fullName }}
        </h3>
      </div>

      <!-- Book Count Badge -->
      <BaseBadge
        v-if="bookCount !== undefined"
        variant="accent"
        size="sm"
      >
        {{ bookCount }}
      </BaseBadge>
    </div>
  </article>
</template>

<script setup>
import { computed } from 'vue'
import BaseBadge from '../base/BaseBadge.vue'

const props = defineProps({
  author: {
    type: Object,
    required: true
  },
  bookCount: {
    type: Number,
    default: undefined
  },
  clickable: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['click'])

const fullName = computed(() => {
  if (props.author.FirstName && props.author.LastName) {
    return `${props.author.LastName} ${props.author.FirstName}`
  }
  return props.author.LastName || props.author.FirstName || 'Unknown Author'
})

const cardClasses = computed(() => {
  const classes = [
    'group', 'border', 'border-gray-200', 'bg-white', 'rounded-lg',
    'shadow-sm', 'transition-all', 'duration-200',
    'animate-snap-in', 'p-4'
  ]

  if (props.clickable) {
    classes.push('hover:shadow-md', 'hover:-translate-y-0.5', 'cursor-pointer')
  }

  return classes
})

const handleClick = () => {
  if (props.clickable) {
    emit('click', props.author)
  }
}
</script>
