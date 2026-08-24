<template>
  <div class="p-6 max-w-5xl mx-auto">
    <!-- Author Detail -->
    <template v-if="selectedAuthor">
      <!-- Back Button -->
      <div class="mb-4">
        <BaseButton variant="ghost" size="sm" @click="goBack">
          ← Назад к авторам
        </BaseButton>
      </div>

      <!-- Author Header -->
      <header class="mb-6 border-b border-gray-200 pb-4">
        <div class="flex justify-between items-start gap-4">
          <h1 class="text-2xl font-display font-semibold text-gray-900">
            {{ fullName(selectedAuthor) }}
          </h1>
          <BaseBadge
            v-if="selectedAuthor.BookCount !== undefined"
            variant="accent"
            size="md"
          >
            {{ selectedAuthor.BookCount }}
          </BaseBadge>
        </div>
      </header>

      <!-- Loading State -->
      <div v-if="isLoadingBooks" class="flex justify-center py-16">
        <BaseLoader type="skeleton-list" :count="5" />
      </div>

      <!-- Grouped Books -->
      <div v-else-if="authorBooks.length" class="space-y-3">
        <UniversalBookCard
          v-for="(group, index) in authorBooks"
          :key="index"
          :book="group"
          @download="handleDownload"
        />
      </div>

      <!-- Empty State -->
      <EmptyState
        v-else
        title="НЕТ КНИГ"
        message="У этого автора нет книг"
        icon="📚"
      />
    </template>

    <!-- Authors List -->
    <template v-else>
      <!-- Header -->
      <header class="mb-6 border-b border-gray-200 pb-4">
        <h1 class="text-2xl font-display font-semibold text-gray-900">
          Авторы
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

      <!-- Authors -->
      <div v-else-if="authors.length" class="space-y-3">
        <AuthorCard
          v-for="author in authors"
          :key="author.ID"
          :author="author"
          :book-count="author.BookCount"
          @click="selectAuthor"
        />
      </div>

      <!-- Empty State - No Authors for Letter -->
      <EmptyState
        v-else-if="selectedLetter"
        title="НЕТ АВТОРОВ"
        :message="`На букву «${selectedLetter}» авторов не найдено`"
        icon="👤"
      />

      <!-- Empty State - No Letter Chosen -->
      <EmptyState
        v-else
        title="АВТОРЫ"
        message="Выберите букву, чтобы посмотреть авторов"
        icon="👤"
      />
    </template>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import AuthorCard from '@/components/domain/AuthorCard.vue'
import UniversalBookCard from '@/components/domain/UniversalBookCard.vue'
import EmptyState from '@/components/domain/EmptyState.vue'
import BaseBadge from '@/components/base/BaseBadge.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import BaseLoader from '@/components/base/BaseLoader.vue'
import { api, downloadBook } from '@/api'

// Same alphabet as the OPDS authors index (api/opds.go)
const letters = Array.from('АБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯABCDEFGHIJKLMNOPQRSTUVWXYZ')

const selectedLetter = ref('')
const authors = ref([])
const isLoading = ref(false)
const error = ref(null)

const selectedAuthor = ref(null)
const authorBooks = ref([])
const isLoadingBooks = ref(false)

const selectLetter = async (letter) => {
  if (letter === selectedLetter.value || isLoading.value) return

  // Reset detail state when navigating via letters
  resetDetail()

  selectedLetter.value = letter
  isLoading.value = true
  error.value = null

  try {
    authors.value = await api.getAuthors(letter)
  } catch (err) {
    console.error('Failed to load authors:', err)
    error.value = 'Не удалось загрузить авторов'
    authors.value = []
  } finally {
    isLoading.value = false
  }
}

const selectAuthor = async (author) => {
  selectedAuthor.value = author
  isLoadingBooks.value = true

  try {
    const [detail, books] = await Promise.all([
      api.getAuthorById(author.ID),
      api.getBooksByAuthor(author.ID)
    ])
    // getAuthorById returns the plain author; keep BookCount from the list item
    selectedAuthor.value = { ...detail, BookCount: author.BookCount }
    authorBooks.value = books
  } catch (err) {
    console.error('Failed to load author books:', err)
    authorBooks.value = []
  } finally {
    isLoadingBooks.value = false
  }
}

const goBack = () => {
  resetDetail()
}

const resetDetail = () => {
  selectedAuthor.value = null
  authorBooks.value = []
  isLoadingBooks.value = false
}

const fullName = (author) => {
  if (author.LastName && author.FirstName) {
    return `${author.LastName} ${author.FirstName}`
  }
  return author.LastName || author.FirstName || 'Неизвестный автор'
}

const handleDownload = async (bookId, format) => {
  try {
    await downloadBook(bookId, format)
  } catch (err) {
    console.error('Download failed:', err)
  }
}
</script>
