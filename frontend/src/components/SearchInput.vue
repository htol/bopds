<template>
    <input type="text" v-model="query" :placeholder="placeholder"
        class="mb-4 px-3 py-2 border rounded w-full max-w-md" />
</template>

<script setup>
import { watch, ref } from 'vue'

const props = defineProps({
    modelValue: { type: String, default: '' },
    placeholder: { type: String, default: 'Поиск...' }
})

const emit = defineEmits(['update:modelValue'])

const query = ref(props.modelValue)

// Синхронизируем v-model
watch(query, (val) => emit('update:modelValue', val))
watch(() => props.modelValue, (val) => {
    if (val !== query.value) query.value = val
})
</script>
