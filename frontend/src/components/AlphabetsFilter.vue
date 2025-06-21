<template>
    <div class="space-y-4">
        <!-- Переключатель раскладок -->
        <div class="flex justify-center gap-2">
            <button v-for="layout in ['ru', 'en']" :key="layout" @click="selectedAlphabet = layout" :class="[
                'px-4 py-1 rounded font-medium',
                selectedAlphabet === layout ? 'bg-blue-600 text-white' : 'bg-gray-200'
            ]">
                {{ layout === 'ru' ? 'РУССКИЕ' : 'ENGLISH' }}
            </button>
        </div>

        <!-- Кнопки алфавита -->
        <div class="flex flex-wrap justify-center gap-2">
            <button v-for="letter in currentAlphabet" :key="letter" @click="$emit('select', letter)" :class="[
                'px-3 py-1 rounded',
                selectedLetter === letter ? 'bg-blue-500 text-white' : 'bg-gray-200'
            ]">
                {{ letter }}
            </button>
        </div>
    </div>
</template>

<script setup>
import { ref, watch, computed } from 'vue'

const props = defineProps({
    selectedLetter: String,
    selectedAlphabet: {
        type: String,
        default: 'ru' // или 'en'
    }
})
const emit = defineEmits(['select', 'update:selectedAlphabet'])

const selectedAlphabet = ref(props.selectedAlphabet)

watch(selectedAlphabet, val => emit('update:selectedAlphabet', val))

const alphabets = {
    ru: 'АБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЭЮЯ'.split(''),
    en: 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('')
}

const currentAlphabet = computed(() => alphabets[selectedAlphabet.value])
</script>
