<template>
  <div class="p-6 max-w-5xl mx-auto">
    <!-- Header -->
    <header class="mb-6 border-b border-gray-200 pb-4">
      <h1 class="text-2xl font-display font-semibold text-gray-900">
        Книги
      </h1>
    </header>

    <!-- Letter Bar -->
    <div class="mb-8 flex flex-wrap justify-center gap-1">
      <button
        v-for="letter in letters"
        :key="letter"
        @click="selectLetter(letter)"
        :class="letter === selectedLetter
          ? 'bg-accent-primary text-white border-accent-primary'
          : 'bg-white text-gray-600 border-gray-300 hover:border-accent-primary/50 hover:text-gray-900'"
        class="w-8 h-8 border rounded font-mono text-sm transition-all duration-200"
      >
        {{ letter }}
      </button>
    </div>

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
      />
    </div>

    <!-- Empty State - No Books for Letter -->
    <EmptyState
      v-else-if="selectedLetter"
      title="НЕТ КНИГ"
      :message="`На букву «${selectedLetter}» книг не найдено`"
      icon="📚"
    />

    <!-- Empty State - No Letter Chosen -->
    <EmptyState
      v-else
      title="КНИГИ"
      message="Выберите букву, чтобы посмотреть книги"
      icon="📚"
    />
  </div>
</template>

<script setup>
import { ref } from 'vue'
import UniversalBookCard from '@/components/domain/UniversalBookCard.vue'
import EmptyState from '@/components/domain/EmptyState.vue'
import BaseLoader from '@/components/base/BaseLoader.vue'
import { api, downloadBook } from '@/api'

// Same alphabet as the OPDS authors index (api/opds.go)
const letters = Array.from('АБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯABCDEFGHIJKLMNOPQRSTUVWXYZ')

const selectedLetter = ref('')
const groups = ref([])
const isLoading = ref(false)
const error = ref(null)

const selectLetter = async (letter) => {
  if (letter === selectedLetter.value || isLoading.value) return

  selectedLetter.value = letter
  isLoading.value = true
  error.value = null

  try {
    groups.value = await api.getBooks(letter)
  } catch (err) {
    console.error('Failed to load books:', err)
    error.value = 'Не удалось загрузить книги'
    groups.value = []
  } finally {
    isLoading.value = false
  }
}

const handleDownload = async (bookId, format) => {
  try {
    await downloadBook(bookId, format)
  } catch (err) {
    console.error('Download failed:', err)
  }
}
</script>
