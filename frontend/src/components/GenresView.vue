<template>
    <div class="p-6 max-w-3xl mx-auto">
        <div class="grid justify-center">
            <h2 class="text-2xl font-semibold mb-4 text-gray-800">Жанры</h2>
        </div>

        <div v-if="isLoading" class="text-gray-500 italic mb-4">Loading...</div>

        <div v-else>
            <div v-if="genres.length" class="grid gap-3 mb-4">
                <div v-for="genre in genres" :key="genre"
                    class="p-2 bg-white rounded shadow-sm border border-gray-300">
                    <p class="text-gray-800 font-medium">{{ genre }}</p>
                </div>
            </div>
            <p v-else class="text-gray-500 italic">Empty result</p>
        </div>
    </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api } from '@/api'

const genres = ref([])
const isLoading = ref(false)

const fetchGenres = async () => {
    isLoading.value = true
    try {
        genres.value = await api.getGenres()
    } catch (err) {
        console.error('Ошибка загрузки жанров:', err)
        genres.value = []
    } finally {
        isLoading.value = false
    }
}

onMounted(fetchGenres)
</script>
