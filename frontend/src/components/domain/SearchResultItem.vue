<template>
  <div
    class="flex items-center gap-4 p-4 bg-white rounded-lg border border-gray-200 hover:border-accent-primary/50 hover:shadow-md transition-all duration-200 cursor-pointer"
    @click="$emit('click', result)"
  >
    <!-- Book Icon -->
    <div class="flex-shrink-0 w-10 h-10 flex items-center justify-center rounded-full bg-gray-100">
      <svg class="w-5 h-5 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
      </svg>
    </div>

    <!-- Result Content -->
    <div class="flex-grow min-w-0">
      <!-- Title with highlighting -->
      <div class="text-sm font-medium text-gray-900 truncate">
        <span v-html="highlightedTitle"></span>
      </div>

      <!-- Author with highlighting -->
      <div class="text-sm text-gray-500 truncate mt-0.5">
        <span v-html="highlightedAuthor"></span>
      </div>
    </div>

    <!-- Relevance indicator (optional) -->
    <div class="flex-shrink-0 text-xs text-gray-400">
      {{ formatRank(result.rank) }}
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import DOMPurify from 'dompurify'

const props = defineProps({
  result: {
    type: Object,
    required: true
  },
  searchQuery: {
    type: String,
    required: true
  }
})

defineEmits(['click'])

// Format rank for display
const formatRank = (rank) => {
  if (rank >= 100) return '🔥'
  if (rank >= 50) return '⭐'
  if (rank >= 10) return '✨'
  return ''
}

// Highlight matching text in title (XSS-SAFE)
const highlightedTitle = computed(() => {
  if (!props.searchQuery) return props.result.title
  return highlightMatches(props.result.title, props.searchQuery)
})

// Highlight matching text in author (XSS-SAFE)
const highlightedAuthor = computed(() => {
  if (!props.searchQuery) return props.result.author
  return highlightMatches(props.result.author, props.searchQuery)
})

// Highlight matches with XSS protection
const highlightMatches = (text, query) => {
  if (!text || !query) return text

  // Escape HTML characters first
  const escapedText = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')

  // Split query into tokens
  const tokens = query.split(/\s+/).filter(t => t.length > 0)
  if (tokens.length === 0) return escapedText

  // Create regex pattern to match any token
  const pattern = new RegExp(`(${tokens.map(t => escapeRegex(t)).join('|')})`, 'gi')

  // Replace matches with highlighted version
  const highlighted = escapedText.replace(pattern, '<mark class="bg-accent-primary/20 text-accent-primary px-1 rounded">$1</mark>')

  // SANITIZE to prevent XSS attacks
  return DOMPurify.sanitize(highlighted, {
    ALLOWED_TAGS: ['mark'],
    ALLOWED_ATTR: ['class']
  })
}

// Escape special regex characters
const escapeRegex = (str) => {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}
</script>

<style scoped>
mark {
  font-weight: 600;
}
</style>
