<template>
  <div class="p-6 max-w-5xl mx-auto">
    <!-- Header - Hide in author mode -->
    <header v-if="mode === 'alphabet'" class="mb-6 border-b border-gray-200 pb-4">
      <div class="flex justify-between items-start gap-4">
        <h1 class="text-2xl font-display font-semibold text-gray-900">
          Книги
        </h1>
        <BaseBadge v-if="filteredBooks.length" variant="accent" size="md">
          {{ filteredBooks.length }}
        </BaseBadge>
      </div>
    </header>

    <!-- Author Mode Header -->
    <div v-if="mode === 'author' && author" class="mb-6">
      <div class="flex items-center gap-4">
        <button
          @click="handleBack"
          class="flex items-center gap-2 text-gray-600 hover:text-gray-900 transition-colors"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
          </svg>
          Назад
        </button>
        <div class="h-6 w-px bg-gray-300"></div>
        <h1 class="text-2xl font-display font-semibold text-gray-900">
          Книги автора: <span class="text-accent-primary">{{ authorFullName }}</span>
        </h1>
        <BaseBadge v-if="filteredBooks.length" variant="accent" size="md">
          {{ filteredBooks.length }}
        </BaseBadge>
      </div>
    </div>

    <!-- Filter Section - Hide in author mode -->
    <section v-if="mode === 'alphabet'" class="mb-6">
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
import { ref, computed, onMounted, watch } from 'vue'
import AlphabetsFilter from '@/components/AlphabetsFilter.vue'
import Paginator from '@/components/Paginator.vue'
import SearchInput from '@/components/SearchInput.vue'
import BookCard from '@/components/domain/BookCard.vue'
import EmptyState from '@/components/domain/EmptyState.vue'
import BaseBadge from '@/components/base/BaseBadge.vue'
import BaseLoader from '@/components/base/BaseLoader.vue'
import { api, downloadBook } from '@/api'

const props = defineProps({
  mode: {
    type: String,
    default: 'alphabet' // 'alphabet' | 'author'
  },
  author: {
    type: Object,
    default: null
  },
  savedState: {
    type: Object,
    default: null
  }
})

const emit = defineEmits(['back-to-alphabet'])

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

const authorFullName = computed(() => {
  if (!props.author) return ''
  const { FirstName, MiddleName, LastName } = props.author
  return `${LastName} ${FirstName} ${MiddleName || ''}`.trim()
})

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

const fetchBooksByAuthor = async () => {
  if (!props.author) return

  isLoading.value = true
  try {
    books.value = await api.getBooksByAuthor(props.author.ID)
    currentPage.value = 1
  } catch (err) {
    console.error('Ошибка загрузки книг автора:', err)
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

const handleBack = () => {
  emit('back-to-alphabet', props.savedState)
}

// Watch for mode changes after initial mount
watch(() => props.mode, (newMode) => {
  if (newMode === 'author' && props.author) {
    fetchBooksByAuthor()
  }
})

// Initial load
onMounted(() => {
  if (props.mode === 'author' && props.author) {
    fetchBooksByAuthor()
  } else {
    fetchBooks()
  }
})
</script>
