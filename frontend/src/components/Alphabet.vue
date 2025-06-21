<template>
    <div class="flex flex-wrap gap-2 mb-4">
        <button v-for="letter in alphabet" :key="letter" @click="selectLetter(letter)"
            :class="['px-3 py-1 rounded', selectedLetter === letter ? 'bg-blue-500 text-white' : 'bg-gray-200']">
            {{ letter }}
        </button>
    </div>

    <ul v-if="authors.length">
        <li v-for="author in authors" :key="author.id">{{ author.name }}</li>
    </ul>
    <p v-else>Выберите букву для отображения авторов.</p>
</template>

<script setup>
import { ref } from 'vue'
import axios from 'axios'

const alphabet = 'АБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЭЮЯ'.split('')
const selectedLetter = ref(null)
const authors = ref([])

const selectLetter = async (letter) => {
    selectedLetter.value = letter
    const response = await axios.get(`http://localhost:3001/api/authors?startsWith=${letter}`)
    authors.value = response.data
}
</script>
