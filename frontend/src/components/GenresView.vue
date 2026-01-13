<template>
  <div class="p-6 max-w-5xl mx-auto">
    <!-- Header -->
    <header class="mb-6 border-b border-gray-200 pb-4">
      <div class="flex justify-between items-start gap-4">
        <h1 class="text-2xl font-display font-semibold text-gray-900">
          Жанры
        </h1>
        <BaseBadge v-if="genres.length" variant="accent" size="md">
          {{ genres.length }}
        </BaseBadge>
      </div>
    </header>

    <!-- Loading State -->
    <div v-if="isLoading" class="flex justify-center py-16">
      <BaseLoader type="skeleton-list" :count="8" />
    </div>

    <!-- Genres List -->
    <div v-else class="space-y-3">
      <div
        v-for="(genre, index) in genres"
        :key="genre"
        @click="$emit('select-genre', genre)"
        class="group border border-gray-200 bg-white rounded-lg p-4 shadow-sm transition-all duration-200 hover:shadow-md hover:-translate-y-0.5 cursor-pointer animate-snap-in"
        :style="{ animationDelay: `${index * 30}ms` }"
      >
        <p class="text-lg font-display font-semibold text-gray-900 group-hover:text-accent-primary transition-colors duration-200">
          {{ genre }}
        </p>
      </div>

      <!-- Empty State -->
      <EmptyState
        v-if="genres.length === 0"
        title="НЕТ ЖАНРОВ"
        message="Не удалось загрузить список жанров"
        icon="∅"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api } from '@/api'
import EmptyState from '@/components/domain/EmptyState.vue'
import BaseBadge from '@/components/base/BaseBadge.vue'
import BaseLoader from '@/components/base/BaseLoader.vue'

const genres = ref([])
const isLoading = ref(false)

const fetchGenres = async () => {
  isLoading.value = true
  try {
    genres.value = await api.getGenres()
  } catch (err) {
    console.error('Ошибка загрузки жанров:', err)
    genres.value = []
  } finally {
    isLoading.value = false
  }
}

const emit = defineEmits(['select-genre'])

onMounted(fetchGenres)
</script>

