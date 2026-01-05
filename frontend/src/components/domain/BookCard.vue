<template>
  <article
    :class="cardClasses"
    @click="handleCardClick"
  >
    <div class="flex justify-between items-start gap-6">
      <!-- Title and Author -->
      <div class="flex-1 min-w-0">
        <h3 class="text-lg font-display font-semibold text-gray-900 mb-1 truncate group-hover:text-accent-primary transition-colors duration-200">
          {{ book.Title }}
        </h3>
        <p v-if="book.Author" class="text-sm text-gray-500">
          {{ book.Author }}
        </p>
      </div>

      <!-- Download Buttons -->
      <div class="flex gap-2 shrink-0">
        <BaseButton
          :variant="isDownloading('fb2') ? 'accent' : 'primary'"
          size="sm"
          :loading="isDownloading('fb2')"
          :disabled="isDownloading('fb2.zip') || isDownloading('epub') || isDownloading('mobi')"
          @click.stop="handleDownload('fb2')"
        >
          {{ isDownloading('fb2') ? '' : 'FB2' }}
        </BaseButton>

        <BaseButton
          :variant="isDownloading('fb2.zip') ? 'accent' : 'primary'"
          size="sm"
          :loading="isDownloading('fb2.zip')"
          :disabled="isDownloading('fb2') || isDownloading('epub') || isDownloading('mobi')"
          @click.stop="handleDownload('fb2.zip')"
        >
          {{ isDownloading('fb2.zip') ? '' : 'FB2.ZIP' }}
        </BaseButton>

        <BaseButton
          :variant="isDownloading('epub') ? 'accent' : 'primary'"
          size="sm"
          :loading="isDownloading('epub')"
          :disabled="isDownloading('fb2') || isDownloading('fb2.zip') || isDownloading('mobi')"
          @click.stop="handleDownload('epub')"
        >
          {{ isDownloading('epub') ? '' : 'EPUB' }}
        </BaseButton>

        <BaseButton
          :variant="isDownloading('mobi') ? 'accent' : 'primary'"
          size="sm"
          :loading="isDownloading('mobi')"
          :disabled="isDownloading('fb2') || isDownloading('fb2.zip') || isDownloading('epub')"
          @click.stop="handleDownload('mobi')"
        >
          {{ isDownloading('mobi') ? '' : 'MOBI' }}
        </BaseButton>
      </div>
    </div>

    <!-- Progress Bar -->
    <div
      v-if="downloading"
      class="absolute bottom-0 left-0 h-1 bg-accent-primary transition-all duration-300"
      :style="{ width: `${downloadProgress}%` }"
    ></div>
  </article>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import BaseButton from '../base/BaseButton.vue'

const props = defineProps({
  book: {
    type: Object,
    required: true
  },
  inverted: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['download'])

const downloading = ref(null)
const downloadProgress = ref(0)
const justMounted = ref(true)

onMounted(() => {
  setTimeout(() => {
    justMounted.value = false
  }, 100)
})

const cardClasses = computed(() => {
  return [
    'relative', 'group', 'border', 'border-gray-200',
    'bg-white', 'rounded-lg', 'p-5',
    'shadow-sm', 'transition-all', 'duration-200',
    'hover:shadow-md', 'hover:-translate-y-0.5',
    'animate-snap-in'
  ]
})

const isDownloading = (format) => {
  return downloading.value === format
}

const handleCardClick = () => {
  // Optional: Make card clickable for book details
  // emit('click', props.book)
}

const handleDownload = async (format) => {
  downloading.value = format
  downloadProgress.value = 0

  // Simulate progress
  const interval = setInterval(() => {
    downloadProgress.value += 10
    if (downloadProgress.value >= 100) {
      clearInterval(interval)
    }
  }, 200)

  try {
    await emit('download', props.book.book_id, format)
  } finally {
    setTimeout(() => {
      downloading.value = null
      downloadProgress.value = 0
    }, 500)
  }
}
</script>
