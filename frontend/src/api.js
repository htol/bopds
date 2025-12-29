const BASE_URL = import.meta.env.VITE_API_BASE_URL || ''

export const fetchAPI = async (endpoint, options = {}) => {
  const res = await fetch(`${BASE_URL}${endpoint}`, options)
  if (!res.ok) throw new Error(res.statusText)
  return res.json()
}

export const api = {
  getGenres: () => fetchAPI('/api/genres'),
  getAuthors: (letter) => fetchAPI(`/api/authors?startsWith=${letter}`),
  getBooks: (letter) => fetchAPI(`/api/books?startsWith=${letter}`)
}
