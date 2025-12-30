<template>
  <div class="p-6 max-w-5xl mx-auto">
    <!-- Header -->
    <header class="mb-6 border-b border-gray-200 pb-4">
      <div class="flex justify-between items-start gap-4">
        <h1 class="text-2xl font-display font-semibold text-gray-900">
          Авторы
        </h1>
        <BaseBadge v-if="filteredAuthors.length" variant="accent" size="md">
          {{ filteredAuthors.length }}
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
        placeholder="Поиск по имени автора..."
        @update:modelValue="currentPage = 1"
      />
    </div>

    <!-- Loading State -->
    <div v-if="isLoading" class="flex justify-center py-16">
      <BaseLoader type="skeleton-list" :count="5" />
    </div>

    <!-- Authors List -->
    <div v-else class="space-y-3 mb-8">
      <AuthorCard
        v-for="(author, index) in paginatedAuthors"
        :key="author.ID"
        :author="author"
        :book-count="author.BookCount"
        :clickable="true"
        :style="{ animationDelay: `${index * 30}ms` }"
        @click="handleAuthorClick"
      />

      <!-- Empty State -->
      <EmptyState
        v-if="paginatedAuthors.length === 0"
        title="НЕТ АВТОРОВ"
        :message="searchQuery ? 'Попробуйте изменить запрос' : 'Выберите другую букву алфавита'"
        icon="∅"
      />
    </div>

    <!-- Paginator -->
    <Paginator
      v-if="totalPages > 1"
      :total-items="filteredAuthors.length"
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
import AuthorCard from '@/components/domain/AuthorCard.vue'
import EmptyState from '@/components/domain/EmptyState.vue'
import BaseBadge from '@/components/base/BaseBadge.vue'
import BaseLoader from '@/components/base/BaseLoader.vue'
import { api } from '@/api'
import { useLibraryStore } from '@/stores/libraryStore'

const { setAuthorMode } = useLibraryStore()
const emit = defineEmits(['show-author-books'])

const authors = ref([])
const selectedLetter = ref('А')
const alphabet = ref('ru')
const searchQuery = ref('')

const isLoading = ref(false)

const currentPage = ref(1)
const pageSize = 10

const filteredAuthors = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return authors.value
  return authors.value.filter((a) =>
    `${a.LastName} ${a.FirstName}`.toLowerCase().includes(q)
  )
})

const paginatedAuthors = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filteredAuthors.value.slice(start, start + pageSize)
})

const totalPages = computed(() => Math.ceil(filteredAuthors.value.length / pageSize))

const fetchAuthors = async () => {
  isLoading.value = true
  try {
    authors.value = await api.getAuthors(selectedLetter.value)
    currentPage.value = 1
  } catch (err) {
    console.error('Ошибка загрузки авторов:', err)
    authors.value = []
  } finally {
    isLoading.value = false
  }
}

const selectLetter = (letter) => {
  selectedLetter.value = letter
  fetchAuthors()
}

const handleAuthorClick = (author) => {
  setAuthorMode(author)
  emit('show-author-books', author)
}

onMounted(fetchAuthors)
</script>

