<template>
  <div class="space-y-4">
    <!-- Alphabet Toggle (RU/EN) -->
    <div class="flex justify-center">
      <div class="inline-flex border border-gray-300 rounded overflow-hidden">
        <button
          v-for="layout in ['ru', 'en']"
          :key="layout"
          @click="selectedAlphabet = layout"
          :class="layoutButtonClasses(layout)"
          class="px-4 py-1.5 font-medium text-sm transition-colors duration-200"
        >
          {{ layout === 'ru' ? 'РУССКИЕ' : 'ENGLISH' }}
        </button>
      </div>
    </div>

    <!-- Letter Buttons - Two Rows -->
    <div class="border border-gray-200 rounded p-4 bg-gray-50">
      <!-- First Row -->
      <div class="flex justify-center gap-1.5 mb-1.5">
        <button
          v-for="letter in firstRow"
          :key="letter"
          @click="$emit('select', letter)"
          :class="letterButtonClasses(letter)"
          class="min-w-[44px] px-3 py-2.5 font-semibold text-base rounded transition-colors duration-200"
        >
          {{ letter }}
        </button>
      </div>
      <!-- Second Row -->
      <div class="flex justify-center gap-1.5">
        <button
          v-for="letter in secondRow"
          :key="letter"
          @click="$emit('select', letter)"
          :class="letterButtonClasses(letter)"
          class="min-w-[44px] px-3 py-2.5 font-semibold text-base rounded transition-colors duration-200"
        >
          {{ letter }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, computed } from 'vue'

const props = defineProps({
  selectedLetter: String,
  selectedAlphabet: {
    type: String,
    default: 'ru'
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

// Split alphabet into two roughly equal rows
const firstRow = computed(() => {
  const alphabet = currentAlphabet.value
  const mid = Math.ceil(alphabet.length / 2)
  return alphabet.slice(0, mid)
})

const secondRow = computed(() => {
  const alphabet = currentAlphabet.value
  const mid = Math.ceil(alphabet.length / 2)
  return alphabet.slice(mid)
})

const layoutButtonClasses = (layout) => {
  if (selectedAlphabet.value === layout) {
    return 'bg-accent-primary text-white'
  } else {
    return 'bg-white text-gray-700 hover:bg-gray-50'
  }
}

const letterButtonClasses = (letter) => {
  if (props.selectedLetter === letter) {
    return 'bg-accent-primary text-white'
  } else {
    return 'bg-white text-gray-700 hover:bg-gray-100 hover:text-accent-primary'
  }
}
</script>

