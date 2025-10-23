<template>
    <div class="p-6 max-w-3xl mx-auto">
        <div class="grid justify-center">
            <h2 class="text-2xl font-semibold mb-4 text-gray-800">Алфавитный указатель фамилий авторов</h2>
        </div>
        <div class="grid p-6">
            <AlphabetsFilter v-model:selectedAlphabet="alphabet" :selectedLetter="selectedLetter"
                @select="selectLetter" />
        </div>

        <!-- Search -->
        <SearchInput v-model="searchQuery" placeholder="Filter results..." @update:modelValue="currentPage = 1" />

        <!-- Loader -->
        <div v-if="isLoading" class="text-gray-500 italic mb-4">Loading..</div>

        <!-- Books -->
        <div v-else>
            <div v-if="paginatedBooks.length" class="grid gap-3 mb-4">
                <div v-for="book in paginatedBooks" :key="book.id"
                    class="p-2 bg-white rounded shadow-sm border border-gray-300">
                    <p class="text-gray-800 font-medium">
                        {{ book.Title }}
                    </p>
                </div>
            </div>

            <p v-else class="text-gray-500 italic">Empty result</p>
        </div>

        <!-- Paginator -->
        <Paginator :total-items="filteredBooks.length" v-model:currentPage="currentPage" :page-size="pageSize" />
    </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import AlphabetsFilter from '@/components/AlphabetsFilter.vue'
import Paginator from '@/components/Paginator.vue'
import SearchInput from '@/components/SearchInput.vue'

const books = ref([])
const selectedLetter = ref('А')
watch(selectedLetter, () => { Object.assign(searchQuery, ref('')) });
const searchQuery = ref('')

const isLoading = ref(false)

const currentPage = ref(1)
const pageSize = 10
const totalBooks = computed(() => books.value.length)
const paginatedBooks = computed(() => {
    const start = (currentPage.value - 1) * pageSize
    return filteredBooks.value.slice(start, start + pageSize)
})

// Фильтрация по названию
const filteredBooks = computed(() => {
    const q = searchQuery.value.trim().toLowerCase()
    if (!q) return books.value
    return books.value.filter((b) =>
        `${b.Title}`.toLowerCase().includes(q)
    )
})

// Запрос книг
const fetchBooks = async () => {
    isLoading.value = true
    try {
        const res = await fetch(`http://localhost:3001/api/books?startsWith=${selectedLetter.value}`)
        if (!res.ok) throw new Error(res.statusText)
        const data = await res.json()
        books.value = data
        currentPage.value = 1
    } catch (err) {
        console.error('Ошибка загрузки книг:', err)
        books.value = []
    } finally {
        isLoading.value = false
    }
}

// Выбор буквы
const selectLetter = (letter) => {
    selectedLetter.value = letter
    fetchBooks()
}

// Стили кнопок
const buttonClass = (letter) => [
    'px-3 py-1 rounded text-sm transition-all',
    selectedLetter.value === letter
        ? 'bg-blue-600 text-white shadow'
        : 'bg-gray-100 hover:bg-gray-200 text-gray-700'
]

onMounted(fetchBooks)
</script>
