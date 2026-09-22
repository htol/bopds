<template>
  <div class="space-y-6">
    <!-- Tab Navigation -->
    <nav class="border-b border-gray-200">
      <div class="flex justify-center">
        <button
          v-for="tab in tabs"
          :key="tab"
          @click="selectTab(tab)"
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

    <!-- Content: the series view replaces the tab content until closed -->
    <SeriesView
      v-if="activeSeries"
      :series="activeSeries"
      @back="closeSeriesView"
      @author-click="handleAuthorClick"
      @series-click="handleSeriesClick"
    />
    <SearchView
      v-else-if="activeTab === 'Поиск'"
      :initial-query="pendingSearch"
      @author-click="handleAuthorClick"
      @series-click="handleSeriesClick"
    />
    <BooksView
      v-else-if="activeTab === 'Книги'"
      @author-click="handleAuthorClick"
      @series-click="handleSeriesClick"
    />
    <AuthorsView
      v-else-if="activeTab === 'Авторы'"
      :initial-author-id="pendingAuthorId"
      @author-click="handleAuthorClick"
      @series-click="handleSeriesClick"
    />
    <GenresView
      v-else-if="activeTab === 'Жанры'"
      @select-genre="handleSelectGenre"
    />
  </div>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'

import SearchView from '@/components/SearchView.vue'
import BooksView from '@/components/BooksView.vue'
import AuthorsView from '@/components/AuthorsView.vue'
import GenresView from '@/components/GenresView.vue'
import SeriesView from '@/components/SeriesView.vue'

const tabs = ['Поиск', 'Книги', 'Авторы', 'Жанры']
const activeTab = ref('Поиск')
const pendingSearch = ref('')
const pendingAuthorId = ref(null)
const activeSeries = ref(null) // { id, name } while the series view is open

const selectTab = (tab) => {
  activeSeries.value = null
  activeTab.value = tab
}

const handleSelectGenre = (genre) => {
  pendingSearch.value = genre
  activeSeries.value = null
  activeTab.value = 'Поиск'
}

// Author click: open the author's book list in the Authors tab detail view
const handleAuthorClick = ({ id }) => {
  pendingAuthorId.value = id
  activeSeries.value = null
  activeTab.value = 'Авторы'
}

// Series click: show the series view in place of the tab content
const handleSeriesClick = (series) => {
  activeSeries.value = series
}

// Back from the series view: return to the previously active tab
const closeSeriesView = () => {
  activeSeries.value = null
}

const tabClasses = (tab) => {
  if (activeTab.value === tab) {
    return 'text-accent-primary border-b-2 border-transparent'
  } else {
    return 'text-gray-600 hover:text-gray-900 border-b-2 border-transparent'
  }
}

// Tab name → URL hash
const tabHashes = {
  'Поиск': 'search',
  'Книги': 'books',
  'Авторы': 'authors',
  'Жанры': 'genres'
}

// Update URL hash when switching tabs
watch(activeTab, (newTab) => {
  const query = newTab === 'Поиск' && pendingSearch.value
    ? `?q=${encodeURIComponent(pendingSearch.value)}`
    : ''
  history.replaceState({ tab: newTab }, '', `#${tabHashes[newTab]}${query}`)

  // Clear pending search after tab switch is handled
  if (newTab !== 'Поиск') {
    pendingSearch.value = ''
  }
  // The pending author id is consumed by AuthorsView on arrival
  if (newTab !== 'Авторы') {
    pendingAuthorId.value = null
  }
})

// Handle browser back button
const handlePopState = (event) => {
  const state = event.state
  if (state && state.tab) {
    activeSeries.value = null
    activeTab.value = state.tab
  }
}

onMounted(() => {
  window.addEventListener('popstate', handlePopState)

  // Set initial state based on current hash
  const hash = window.location.hash
  if (hash.includes('#books')) {
    activeTab.value = 'Книги'
  } else if (hash.includes('#authors')) {
    activeTab.value = 'Авторы'
  } else if (hash.includes('#genres')) {
    activeTab.value = 'Жанры'
  } else {
    activeTab.value = 'Поиск'
  }

  // Set initial history state
  history.replaceState({ tab: activeTab.value }, '', `#${tabHashes[activeTab.value]}`)
})
</script>
