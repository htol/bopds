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
    <component :is="currentComponent" />
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

import AuthorsView from '@/components/AuthorsView.vue'
import GenresView from '@/components/GenresView.vue'
import BooksView from '@/components/BooksView.vue'

const tabs = ['Авторы', 'Жанры', 'Книги']
const activeTab = ref('Авторы')

const currentComponent = computed(() => {
  switch (activeTab.value) {
    case 'Авторы':
      return AuthorsView
    case 'Жанры':
      return GenresView
    case 'Книги':
      return BooksView
    default:
      return AuthorsView
  }
})

const tabClasses = (tab) => {
  if (activeTab.value === tab) {
    return 'text-accent-primary border-b-2 border-transparent'
  } else {
    return 'text-gray-600 hover:text-gray-900 border-b-2 border-transparent'
  }
}
</script>
