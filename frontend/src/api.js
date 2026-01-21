const BASE_URL = import.meta.env.VITE_API_BASE_URL || ''

export const fetchAPI = async (endpoint, options = {}) => {
  const res = await fetch(`${BASE_URL}${endpoint}`, options)
  if (!res.ok) throw new Error(res.statusText)
  return res.json()
}

// Download a book file (FB2 or EPUB)
export const downloadBook = async (bookId, format = 'fb2') => {
  const res = await fetch(`${BASE_URL}/api/books/${bookId}/download?format=${format}`)

  // Extract filename from Content-Disposition header
  const contentDisposition = res.headers.get('Content-Disposition')
  let filename = `book.${format}`
  if (contentDisposition) {
    // Try RFC 5987 encoding first (filename*=UTF-8''...)
    const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i)
    if (utf8Match && utf8Match[1]) {
      filename = decodeURIComponent(utf8Match[1])
    } else {
      // Fallback to regular filename="..."
      const filenameMatch = contentDisposition.match(/filename="([^"]+)"/)
      if (filenameMatch && filenameMatch[1]) {
        filename = filenameMatch[1]
      }
    }
  }

  if (!res.ok) {
    const errorText = await res.text()
    throw new Error(errorText || res.statusText)
  }

  const blob = await res.blob()
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  window.URL.revokeObjectURL(url)
  document.body.removeChild(a)
}

export const api = {
  getGenres: () => fetchAPI('/api/genres'),
  getAuthors: (letter) => fetchAPI(`/api/authors?startsWith=${letter}`),
  getBooks: (letter) => fetchAPI(`/api/books?startsWith=${letter}`),
  getBooksByAuthor: (authorId) => fetchAPI(`/api/authors/${authorId}/books`),
  getAuthorById: (authorId) => fetchAPI(`/api/authors/${authorId}`),
  searchBooks: (query, limit = 20, offset = 0, fields = []) => {
    let url = `/api/search?q=${encodeURIComponent(query)}&limit=${limit}&offset=${offset}`
    if (fields && fields.length > 0) {
      url += `&fields=${fields.join(',')}`
    }
    return fetchAPI(url)
  }
}
