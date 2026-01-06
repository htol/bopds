<template>
  <div class="space-y-6">
    <!-- Tab Navigation -->
    <nav class="border-b border-gray-200">
      <div class="flex justify-center">
        <button
          v-for="tab in tabs"
          :key="tab"
          @click="activeTab = tab"
          :class="tabClasses(tab)"
          class="relative px-6 py-3 font-display font-medium text-base transition-all duration-200"
        >
          {{ tab }}

          <!-- Active indicator -->
          <div
            v-if="activeTab === tab"
            class="absolute bottom-0 left-0 right-0 h-0.5 bg-accent-primary"
          ></div>
        </button>
      </div>
    </nav>

    <!-- Content -->
    <SearchView v-if="activeTab === 'Поиск'" />
    <AuthorsView
      v-if="activeTab === 'Авторы'"
      @show-author-books="handleShowAuthorBooks"
    />
    <GenresView v-if="activeTab === 'Жанры'" />
    <BooksView
      v-if="activeTab === 'Книги'"
      :mode="booksViewMode"
      :author="currentAuthor"
      :saved-state="savedBooksState"
      @back-to-alphabet="handleBackToAlphabet"
    />
  </div>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'

import SearchView from '@/components/SearchView.vue'
import AuthorsView from '@/components/AuthorsView.vue'
import GenresView from '@/components/GenresView.vue'
import BooksView from '@/components/BooksView.vue'

const tabs = ['Поиск', 'Авторы', 'Жанры', 'Книги']
const activeTab = ref('Поиск')

// Books view state
const booksViewMode = ref('alphabet')
const currentAuthor = ref(null)
const savedBooksState = ref(null)

const tabClasses = (tab) => {
  if (activeTab.value === tab) {
    return 'text-accent-primary border-b-2 border-transparent'
  } else {
    return 'text-gray-600 hover:text-gray-900 border-b-2 border-transparent'
  }
}

const handleShowAuthorBooks = (author) => {
  // Push state to browser history
  const state = { from: 'author', authorId: author.ID, tab: 'Авторы' }
  history.pushState(state, '', `#books?author=${author.ID}`)

  // Switch to Books tab
  activeTab.value = 'Книги'
  booksViewMode.value = 'author'
  currentAuthor.value = author
}

const handleBackToAlphabet = (savedState) => {
  // Switch back to Authors tab
  activeTab.value = 'Авторы'
  // Reset books view to alphabet mode
  booksViewMode.value = 'alphabet'
  currentAuthor.value = null
  savedBooksState.value = savedState

  // Clear hash
  history.pushState({ tab: 'Авторы' }, '', '#authors')
}

// Reset books mode when switching tabs (including staying on Books but switching away from author mode)
watch(activeTab, (newTab, oldTab) => {
  // Only reset if we're not switching from Authors (which sets author mode)
  if (newTab !== 'Книги') {
    booksViewMode.value = 'alphabet'
    currentAuthor.value = null
  } else if (newTab === 'Книги' && oldTab === 'Книги' && booksViewMode.value === 'author') {
    // User clicked Books tab while already on Books tab in author mode - reset it
    booksViewMode.value = 'alphabet'
    currentAuthor.value = null
  }

  // Update URL hash
  if (newTab === 'Поиск') {
    history.replaceState({ tab: newTab }, '', '#search')
  } else if (newTab === 'Авторы') {
    history.replaceState({ tab: newTab }, '', '#authors')
  } else if (newTab === 'Жанры') {
    history.replaceState({ tab: newTab }, '', '#genres')
  } else if (newTab === 'Книги') {
    history.replaceState({ tab: newTab }, '', '#books')
  }
})

// Handle browser back button
const handlePopState = (event) => {
  const state = event.state
  if (!state) return

  if (state.tab) {
    activeTab.value = state.tab
  }

  if (state.from === 'author' && state.tab === 'Авторы') {
    // Returning from author view to authors
    booksViewMode.value = 'alphabet'
    currentAuthor.value = null
  }
}

onMounted(() => {
  window.addEventListener('popstate', handlePopState)

  // Set initial state based on current hash
  const hash = window.location.hash
  if (hash.includes('#search')) {
    activeTab.value = 'Поиск'
  } else if (hash.includes('#books')) {
    activeTab.value = 'Книги'
  } else if (hash.includes('#genres')) {
    activeTab.value = 'Жанры'
  } else if (hash.includes('#authors')) {
    activeTab.value = 'Авторы'
  }

  // Set initial history state
  const tabLower = activeTab.value.toLowerCase()
  let hashStr = ''
  if (tabLower === 'поиск') {
    hashStr = 'search'
  } else if (tabLower === 'авторы') {
    hashStr = 'authors'
  } else if (tabLower === 'жанры') {
    hashStr = 'genres'
  } else {
    hashStr = 'books'
  }
  history.replaceState({ tab: activeTab.value }, '', `#${hashStr}`)
})
</script>
