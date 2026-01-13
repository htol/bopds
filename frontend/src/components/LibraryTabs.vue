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
    <SearchView v-if="activeTab === 'Поиск'" :initial-query="pendingSearch" />
    <GenresView v-if="activeTab === 'Жанры'" @select-genre="handleSelectGenre" />
  </div>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'

import SearchView from '@/components/SearchView.vue'
import GenresView from '@/components/GenresView.vue'

const tabs = ['Поиск', 'Жанры']
const activeTab = ref('Поиск')
const pendingSearch = ref('')

const handleSelectGenre = (genre) => {
  pendingSearch.value = genre
  activeTab.value = 'Поиск'
}

const tabClasses = (tab) => {
  if (activeTab.value === tab) {
    return 'text-accent-primary border-b-2 border-transparent'
  } else {
    return 'text-gray-600 hover:text-gray-900 border-b-2 border-transparent'
  }
}

// Update URL hash when switching tabs
watch(activeTab, (newTab) => {
  if (newTab === 'Поиск') {
    history.replaceState({ tab: newTab }, '', '#search' + (pendingSearch.value ? `?q=${encodeURIComponent(pendingSearch.value)}` : ''))
  } else if (newTab === 'Жанры') {
    history.replaceState({ tab: newTab }, '', '#genres')
  }

  // Clear pending search after tab switch is handled
  if (newTab !== 'Поиск') {
    pendingSearch.value = ''
  }
})

// Handle browser back button
const handlePopState = (event) => {
  const state = event.state
  if (state && state.tab) {
    activeTab.value = state.tab
  }
}

onMounted(() => {
  window.addEventListener('popstate', handlePopState)

  // Set initial state based on current hash
  const hash = window.location.hash
  if (hash.includes('#genres')) {
    activeTab.value = 'Жанры'
  } else {
    activeTab.value = 'Поиск'
  }

  // Set initial history state
  const tabLower = activeTab.value.toLowerCase()
  let hashStr = 'search'
  if (tabLower === 'жанры') {
    hashStr = 'genres'
  }
  history.replaceState({ tab: activeTab.value }, '', `#${hashStr}`)
})
</script>
