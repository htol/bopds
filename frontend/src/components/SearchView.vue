<template>
  <div class="p-6 max-w-5xl mx-auto">
    <!-- Search Input Section -->
    <div class="mb-8">
      <SearchInput
        v-model="searchQuery"
        placeholder="Search books by title or author..."
        @update:modelValue="handleSearch"
      />
    </div>

    <!-- Error State -->
    <div v-if="error" class="mb-6 bg-red-50 border border-red-200 text-red-700 p-4 rounded-lg">
      <div class="flex items-center gap-2">
        <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
          <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
        </svg>
        <span>{{ error }}</span>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="isLoading && results.length === 0" class="flex justify-center py-16">
      <BaseLoader type="skeleton-list" :count="5" />
    </div>

    <!-- Search Results -->
    <div v-else class="space-y-3">
      <UniversalBookCard
        v-for="result in results"
        :key="result.book_id"
        :book="result"
        :search-query="searchQuery"
        @download="handleDownload"
        @click="handleResultClick"
      />

      <!-- Empty State - No Results -->
      <EmptyState
        v-if="results.length === 0 && !isLoading && hasSearched"
        title="НЕТ РЕЗУЛЬТАТОВ"
        :message="`По запросу «${searchQuery}» ничего не найдено`"
        icon="🔍"
      />

      <!-- Initial State (before search) -->
      <EmptyState
        v-if="!hasSearched && results.length === 0"
        title="ПОИСК КНИГ"
        message="Введите название книги или имя автора для поиска"
        icon="📚"
      />
    </div>

    <!-- Loading More Indicator -->
    <div v-if="isLoadingMore" class="flex justify-center py-8">
      <BaseLoader type="spinner" />
    </div>

    <!-- No More Results Indicator -->
    <div v-if="hasNoMoreResults && results.length > 0" class="text-center py-4 text-gray-400 text-sm">
      Все результаты показаны
    </div>

    <!-- Infinite Scroll Sentinel -->
    <div ref="sentinel" class="h-1"></div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, provide } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import SearchInput from '@/components/SearchInput.vue'
import UniversalBookCard from '@/components/domain/UniversalBookCard.vue'
import EmptyState from '@/components/domain/EmptyState.vue'
import BaseLoader from '@/components/base/BaseLoader.vue'
import { api, downloadBook } from '@/api'

// State
const searchQuery = ref('')
const results = ref([])
const isLoading = ref(false)
const isLoadingMore = ref(false)
const hasSearched = ref(false)
const hasNoMoreResults = ref(false)
const error = ref(null)

// Pagination
const page = ref(1)
const pageSize = 20
const sentinel = ref(null)

// Provide search query for child components
provide('searchQuery', searchQuery)

// Debounced search function (300ms delay)
const debouncedSearch = useDebounceFn(async (query) => {
  // Clear previous error
  error.value = null

  if (!query.trim()) {
    results.value = []
    hasSearched.value = false
    hasNoMoreResults.value = false
    page.value = 1
    return
  }

  hasSearched.value = true
  isLoading.value = true
  page.value = 1
  hasNoMoreResults.value = false

  try {
    const newResults = await api.searchBooks(query, pageSize, 0)
    results.value = newResults
    hasNoMoreResults.value = newResults.length < pageSize
  } catch (err) {
    console.error('Search error:', err)
    error.value = err.message || 'Failed to search books'
    results.value = []
  } finally {
    isLoading.value = false
  }
}, 300)

// Handle search input
const handleSearch = (query) => {
  debouncedSearch(query)
}

// Load more results (infinite scroll)
const loadMore = async () => {
  // Guard checks
  if (isLoading.value || isLoadingMore.value || hasNoMoreResults.value) return
  if (!searchQuery.value.trim()) return

  // Set loading flag immediately
  isLoadingMore.value = true
  const nextPage = page.value + 1

  try {
    const offset = page.value * pageSize
    const newResults = await api.searchBooks(searchQuery.value, pageSize, offset)

    if (newResults.length === 0) {
      hasNoMoreResults.value = true
    } else {
      results.value = [...results.value, ...newResults]
      page.value = nextPage
      hasNoMoreResults.value = newResults.length < pageSize
    }
  } catch (err) {
    console.error('Load more error:', err)
    error.value = err.message || 'Failed to load more results'
  } finally {
    isLoadingMore.value = false
  }
}

// Intersection Observer for infinite scroll
let observer = null

const setupIntersectionObserver = () => {
  if (!sentinel.value) return

  observer = new IntersectionObserver(
    (entries) => {
      if (entries[0].isIntersecting) {
        loadMore()
      }
    },
    {
      root: null,
      rootMargin: '100px',
      threshold: 0.1
    }
  )

  observer.observe(sentinel.value)
}

// Handle result click
const handleResultClick = (result) => {
  // TODO: Implement click behavior when needed
  console.log('Clicked result:', result)
}

const handleDownload = async (bookId, format) => {
  try {
    await downloadBook(bookId, format)
  } catch (err) {
    console.error('Download failed:', err)
    // Could show a toast notification here
  }
}

// Lifecycle hooks
onMounted(() => {
  // Setup intersection observer after next tick
  setTimeout(() => {
    setupIntersectionObserver()
  }, 100)
})

onUnmounted(() => {
  // Cancel debounced search (prevents memory leak)
  if (debouncedSearch && debouncedSearch.cancel) {
    debouncedSearch.cancel()
  }
  
  // Disconnect observer
  if (observer) {
    observer.disconnect()
  }
})
</script>
