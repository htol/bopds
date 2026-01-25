<template>
  <article
    :class="cardClasses"
    @click="handleCardClick"
  >
    <div class="flex items-start gap-4">
      <!-- Book Icon / Cover Placeholder -->
      <div class="flex-shrink-0 w-12 h-16 bg-gray-100 rounded flex items-center justify-center text-gray-400">
        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
        </svg>
      </div>

      <!-- Content -->
      <div class="flex-1 min-w-0">
        <!-- Title -->
        <h3 class="text-lg font-display font-semibold text-gray-900 mb-0.5 truncate group-hover:text-accent-primary transition-colors duration-200">
          <span v-html="highlightedTitle"></span>
        </h3>

        <!-- Author -->
        <p class="text-sm text-gray-600 mb-1 truncate">
          <span v-html="highlightedAuthor"></span>
        </p>

        <!-- Meta Info Row -->
        <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500 mb-3">
          <!-- Language -->
          <div v-if="book.lang" class="flex items-center gap-1 w-8 shrink-0">
            <span class="uppercase font-mono bg-gray-100 px-1.5 py-0.5 rounded border border-gray-200">
              {{ book.lang }}
            </span>
          </div>

          <!-- Size -->
          <div v-if="bookFileSize" class="flex items-center gap-1 w-20 shrink-0 whitespace-nowrap">
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
            </svg>
            <span>{{ bookFileSize }}</span>
          </div>

          <!-- Series -->
          <div v-if="bookSeries" class="flex items-center gap-1 text-accent-secondary">
            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
            </svg>
            <span class="font-medium bg-accent-secondary/10 px-1.5 py-0.5 rounded">
              {{ bookSeries }}
            </span>
          </div>
        </div>

        <!-- Download Buttons -->
        <div class="flex flex-wrap gap-2 mt-2 justify-end">
          <BaseButton
            v-for="format in ['fb2', 'fb2.zip', 'epub', 'mobi']"
            :key="format"
            :variant="isDownloading(format) ? 'accent' : 'secondary'"
            size="xs"
            :loading="isDownloading(format)"
            :disabled="hasActiveDownload && !isDownloading(format)"
            class="min-w-[4rem] font-mono"
            @click.stop="handleDownload(format)"
          >
            {{ isDownloading(format) ? '' : format.toUpperCase() }}
          </BaseButton>
        </div>
      </div>
    </div>

    <!-- Progress Bar -->
    <div
      v-if="currentDownloadFormat"
      class="absolute bottom-0 left-0 h-0.5 bg-accent-primary transition-all duration-300"
      :style="{ width: `${downloadProgress}%` }"
    ></div>
  </article>
</template>

<script setup>
import { ref, computed } from 'vue'
import BaseButton from '../base/BaseButton.vue'
import DOMPurify from 'dompurify'

const props = defineProps({
  book: {
    type: Object,
    required: true
  },
  searchQuery: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['download', 'click'])

const currentDownloadFormat = ref(null)
const downloadProgress = ref(0)

const cardClasses = computed(() => {
  return [
    'relative', 'group', 'bg-white', 'rounded-lg', 'p-4',
    'border', 'border-gray-200',
    'hover:border-accent-primary/50', 'hover:shadow-md',
    'transition-all', 'duration-200',
    'cursor-pointer', 'overflow-hidden'
  ]
})

const bookSeries = computed(() => {
  // Handle both flat fields (search) and nested struct (book detail)
  // Search uses: series_name, series_no
  // Book detail (JSON) uses: series.name, series.series_no
  // Book detail (Go struct access) uses: Series.Name, Series.SeriesNo
  const name = props.book.series_name || 
               props.book.Series?.Name || 
               props.book.series?.name
               
  const no = props.book.series_no || 
             props.book.Series?.SeriesNo || 
             props.book.series?.series_no
  
  if (!name) return null
  return no ? `${name} #${no}` : name
})

const bookFileSize = computed(() => {
  const bytes = props.book.file_size || props.book.FileSize
  if (!bytes) return null
  
  const units = ['B', 'KB', 'MB', 'GB']
  let size = bytes
  let unitIndex = 0
  
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex++
  }
  
  return `${size.toFixed(1)} ${units[unitIndex]}`
})

const hasActiveDownload = computed(() => !!currentDownloadFormat.value)

const isDownloading = (format) => currentDownloadFormat.value === format

const handleDownload = async (format) => {
  if (hasActiveDownload.value) return

  currentDownloadFormat.value = format
  downloadProgress.value = 0

  // Simulate progress
  const interval = setInterval(() => {
    if (downloadProgress.value < 90) {
      downloadProgress.value += 10
    }
  }, 200)

  try {
    const bookId = props.book.book_id || props.book.BookID
    await emit('download', bookId, format)
    downloadProgress.value = 100
  } catch (error) {
    console.error('Download failed:', error)
  } finally {
    clearInterval(interval)
    setTimeout(() => {
      currentDownloadFormat.value = null
      downloadProgress.value = 0
    }, 500)
  }
}

const handleCardClick = () => {
  emit('click', props.book)
}

// Reuse highlighting logic from SearchResultItem
const highlightMatches = (text, query) => {
  if (!text) return ''
  if (!query) return text

  const escapedText = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')

  const tokens = query.split(/\s+/).filter(t => t.length > 0)
  if (tokens.length === 0) return escapedText

  const pattern = new RegExp(`(${tokens.map(t => t.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')})`, 'gi')
  const highlighted = escapedText.replace(pattern, '<mark class="bg-accent-primary/20 text-accent-primary px-0.5 rounded">$1</mark>')

  return DOMPurify.sanitize(highlighted, {
    ALLOWED_TAGS: ['mark'],
    ALLOWED_ATTR: ['class']
  })
}

const highlightedTitle = computed(() => {
  return highlightMatches(props.book.Title || props.book.title, props.searchQuery)
})

const highlightedAuthor = computed(() => {
  // Handle both string author (search) and array of authors (detail)
  let authorText = ''
  if (Array.isArray(props.book.Author)) {
    authorText = props.book.Author.map(a => `${a.FirstName || ''} ${a.LastName || ''}`.trim()).join(', ')
  } else {
    authorText = props.book.Author || props.book.author || ''
  }
  return highlightMatches(authorText, props.searchQuery)
})
</script>

<style scoped>
mark {
  background-color: transparent;
  color: inherit;
  font-weight: 700;
}
</style>
