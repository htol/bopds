<template>
  <div class="p-6 max-w-5xl mx-auto">
    <!-- Header -->
    <header class="mb-6 border-b border-gray-200 pb-4">
      <div class="flex justify-between items-start gap-4">
        <h1 class="text-2xl font-display font-semibold text-gray-900">
          Книги
        </h1>
        <BaseBadge v-if="filteredBooks.length" variant="accent" size="md">
          {{ filteredBooks.length }}
        </BaseBadge>
      </div>
    </header>

    <!-- Filter Section -->
    <section class="mb-6">
      <AlphabetsFilter
        v-model:selectedAlphabet="alphabet"
        :selectedLetter="selectedLetter"
        @select="selectLetter"
      />
    </section>

    <!-- Search -->
    <div class="mb-8">
      <SearchInput
        v-model="searchQuery"
        placeholder="Поиск по названию..."
        @update:modelValue="currentPage = 1"
      />
    </div>

    <!-- Loading State -->
    <div v-if="isLoading" class="flex justify-center py-16">
      <BaseLoader type="skeleton-list" :count="5" />
    </div>

    <!-- Books List -->
    <div v-else class="space-y-3 mb-8">
      <BookCard
        v-for="(book, index) in paginatedBooks"
        :key="book.book_id"
        :book="book"
        :style="{ animationDelay: `${index * 30}ms` }"
        @download="handleDownload"
      />

      <!-- Empty State -->
      <EmptyState
        v-if="paginatedBooks.length === 0"
        title="НЕТ КНИГ"
        :message="searchQuery ? 'Попробуйте изменить запрос' : 'Выберите другую букву алфавита'"
        icon="∅"
      />
    </div>

    <!-- Paginator -->
    <Paginator
      v-if="totalPages > 1"
      :total-items="filteredBooks.length"
      v-model:currentPage="currentPage"
      :page-size="pageSize"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import AlphabetsFilter from '@/components/AlphabetsFilter.vue'
import Paginator from '@/components/Paginator.vue'
import SearchInput from '@/components/SearchInput.vue'
import BookCard from '@/components/domain/BookCard.vue'
import EmptyState from '@/components/domain/EmptyState.vue'
import BaseBadge from '@/components/base/BaseBadge.vue'
import BaseLoader from '@/components/base/BaseLoader.vue'
import { api, downloadBook } from '@/api'

const books = ref([])
const selectedLetter = ref('А')
const alphabet = ref('ru')
const searchQuery = ref('')

const isLoading = ref(false)

const currentPage = ref(1)
const pageSize = 10

const filteredBooks = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return books.value
  return books.value.filter((b) =>
    `${b.Title}`.toLowerCase().includes(q)
  )
})

const paginatedBooks = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filteredBooks.value.slice(start, start + pageSize)
})

const totalPages = computed(() => Math.ceil(filteredBooks.value.length / pageSize))

const fetchBooks = async () => {
  isLoading.value = true
  try {
    books.value = await api.getBooks(selectedLetter.value)
    currentPage.value = 1
  } catch (err) {
    console.error('Ошибка загрузки книг:', err)
    books.value = []
  } finally {
    isLoading.value = false
  }
}

const selectLetter = (letter) => {
  selectedLetter.value = letter
  fetchBooks()
}

const handleDownload = async (bookId, format) => {
  try {
    await downloadBook(bookId, format)
  } catch (err) {
    console.error('Download error:', err)
    alert('Failed to download book: ' + err.message)
  }
}

onMounted(fetchBooks)
</script>

