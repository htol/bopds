<template>
  <div class="p-6 max-w-5xl mx-auto">
    <!-- Back Button -->
    <div class="mb-4">
      <BaseButton variant="ghost" size="sm" @click="emit('back')">
        ← Назад
      </BaseButton>
    </div>

    <!-- Series Header -->
    <header class="mb-6 border-b border-gray-200 pb-4">
      <div class="flex justify-between items-start gap-4">
        <h1 class="text-2xl font-display font-semibold text-gray-900">
          {{ series.name }}
        </h1>
        <BaseBadge v-if="groups.length" variant="accent" size="md">
          {{ groups.length }}
        </BaseBadge>
      </div>
    </header>

    <!-- Error State -->
    <div v-if="error" class="mb-6 bg-red-50 border border-red-200 text-red-700 p-4 rounded-lg">
      {{ error }}
    </div>

    <!-- Loading State -->
    <div v-else-if="isLoading" class="flex justify-center py-16">
      <BaseLoader type="skeleton-list" :count="5" />
    </div>

    <!-- Grouped Books -->
    <div v-else-if="groups.length" class="space-y-3">
      <UniversalBookCard
        v-for="(group, index) in groups"
        :key="index"
        :book="group"
        @download="handleDownload"
        @author-click="emit('author-click', $event)"
        @series-click="emit('series-click', $event)"
      />
    </div>

    <!-- Empty State -->
    <EmptyState
      v-else
      title="НЕТ КНИГ"
      message="В этой серии нет книг"
      icon="📚"
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import UniversalBookCard from '@/components/domain/UniversalBookCard.vue'
import EmptyState from '@/components/domain/EmptyState.vue'
import BaseBadge from '@/components/base/BaseBadge.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseLoader from '@/components/base/BaseLoader.vue'
import { api, downloadBook } from '@/api'

const props = defineProps({
  // Series to display: { id, name } from a series-click
  series: {
    type: Object,
    required: true
  }
})

const emit = defineEmits(['back', 'author-click', 'series-click'])

const groups = ref([])
const isLoading = ref(false)
const error = ref(null)

onMounted(async () => {
  isLoading.value = true
  error.value = null

  try {
    groups.value = await api.getBooksBySeries(props.series.id)
  } catch (err) {
    console.error('Failed to load series books:', err)
    error.value = 'Не удалось загрузить книги серии'
    groups.value = []
  } finally {
    isLoading.value = false
  }
})

const handleDownload = async (bookId, format) => {
  try {
    await downloadBook(bookId, format)
  } catch (err) {
    console.error('Download failed:', err)
  }
}
</script>
