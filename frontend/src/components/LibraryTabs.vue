<template>
    <div class="space-y-4">
        <div class="flex justify-center gap-2">
            <button v-for="tab in tabs" :key="tab" @click="activeTab = tab" :class="[
                'px-4 py-2 rounded font-medium',
                activeTab === tab ? 'bg-blue-600 text-white' : 'bg-gray-200'
            ]">
                {{ tab }}
            </button>
        </div>

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
</script>
