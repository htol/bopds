import { reactive } from 'vue'

// Global state for cross-component communication
const state = reactive({
  // Track which mode BooksView is in
  booksViewMode: 'alphabet', // 'alphabet' | 'author'

  // Author-specific state
  currentAuthor: null, // { id, firstName, lastName, ... }

  // Save previous state for back navigation
  savedState: null
})

export function useLibraryStore() {
  const setAuthorMode = (author) => {
    // Save current state before switching
    state.savedState = {
      mode: 'alphabet',
      author: null
    }
    // Set new mode
    state.booksViewMode = 'author'
    state.currentAuthor = author
  }

  const clearAuthorMode = () => {
    state.booksViewMode = 'alphabet'
    state.currentAuthor = null
  }

  const getAuthorMode = () => {
    return {
      mode: state.booksViewMode,
      author: state.currentAuthor
    }
  }

  return {
    state,
    setAuthorMode,
    clearAuthorMode,
    getAuthorMode
  }
}
