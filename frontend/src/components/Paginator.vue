<template>
    <div v-if="totalPages > 1" class="flex justify-center items-center gap-2 mt-4 flex-wrap">
        <button @click="$emit('update:currentPage', currentPage - 1)" :disabled="currentPage === 1"
            class="px-3 py-1 rounded text-sm bg-gray-200 hover:bg-gray-300 disabled:opacity-50">
            ←
        </button>

        <button v-for="page in visiblePages" :key="page" :disabled="page === '...'"
            @click="typeof page === 'number' && $emit('update:currentPage', page)" :class="[
                'px-3 py-1 rounded text-sm',
                page === currentPage ? 'bg-blue-600 text-white' : 'bg-gray-200 hover:bg-gray-300',
                page === '...' ? 'cursor-default bg-transparent' : ''
            ]">
            {{ page }}
        </button>

        <button @click="$emit('update:currentPage', currentPage + 1)" :disabled="currentPage === totalPages"
            class="px-3 py-1 rounded text-sm bg-gray-200 hover:bg-gray-300 disabled:opacity-50">
            →
        </button>
    </div>
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
</script>
