<template>
  <nav v-if="totalPages > 1" class="flex justify-center items-center gap-2 mt-6 flex-wrap">
    <!-- Previous Button -->
    <button
      @click="$emit('update:currentPage', currentPage - 1)"
      :disabled="currentPage === 1"
      class="px-3 py-1.5 font-medium text-sm border border-gray-300 bg-white text-gray-700 rounded transition-colors duration-200 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
    >
      ←
    </button>

    <!-- Page Numbers -->
    <button
      v-for="page in visiblePages"
      :key="page"
      :disabled="page === '...'"
      @click="typeof page === 'number' && $emit('update:currentPage', page)"
      :class="pageButtonClasses(page)"
      class="min-w-[40px] px-3 py-1.5 font-medium text-sm border rounded transition-colors duration-200"
    >
      {{ page }}
    </button>

    <!-- Next Button -->
    <button
      @click="$emit('update:currentPage', currentPage + 1)"
      :disabled="currentPage === totalPages"
      class="px-3 py-1.5 font-medium text-sm border border-gray-300 bg-white text-gray-700 rounded transition-colors duration-200 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
    >
      →
    </button>
  </nav>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  totalItems: { type: Number, required: true },
  currentPage: { type: Number, required: true },
  pageSize: { type: Number, default: 10 }
})

const emits = defineEmits(['update:currentPage'])

const totalPages = computed(() => Math.ceil(props.totalItems / props.pageSize))

const pageRange = 2

const visiblePages = computed(() => {
  const pages = []
  const total = totalPages.value
  const current = props.currentPage

  if (total <= 7) {
    for (let i = 1; i <= total; i++) pages.push(i)
    return pages
  }

  pages.push(1)

  if (current > pageRange + 2) {
    pages.push('...')
  }

  const start = Math.max(2, current - pageRange)
  const end = Math.min(total - 1, current + pageRange)

  for (let i = start; i <= end; i++) {
    pages.push(i)
  }

  if (current < total - pageRange - 1) {
    pages.push('...')
  }

  pages.push(total)

  return pages
})

const pageButtonClasses = (page) => {
  if (page === '...') {
    return 'bg-transparent cursor-default border-transparent text-gray-500'
  } else if (page === props.currentPage) {
    return 'bg-accent-primary text-white border-accent-primary hover:bg-accent-primary'
  } else {
    return 'bg-white text-gray-700 hover:bg-gray-50 border-gray-300 hover:text-accent-primary'
  }
}
</script>

