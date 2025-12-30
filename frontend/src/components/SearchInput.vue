<template>
  <input
    type="text"
    v-model="query"
    :placeholder="placeholder"
    class="w-full px-4 py-2.5 border border-gray-300 bg-white text-gray-900 placeholder:text-gray-400 rounded-lg transition-colors duration-200 focus:border-accent-primary focus:outline-none focus:ring-2 focus:ring-accent-primary/20"
  />
</template>

<script setup>
import { watch, ref } from 'vue'

const props = defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: 'Поиск...' }
})

const emit = defineEmits(['update:modelValue'])

const query = ref(props.modelValue)

watch(query, (val) => emit('update:modelValue', val))
watch(() => props.modelValue, (val) => {
  if (val !== query.value) query.value = val
})
</script>

