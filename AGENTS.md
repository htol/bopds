# AGENTS.md

This file guides agentic coding assistants working in the bopds repository.

## Project Overview

bopds is a Basic OPDS server that serves FB2 books. It consists of:

- **Backend**: Go 1.25.5 with SQLite (CGO), net/http, structured logging
- **Frontend**: Vue 3 + Vite + Tailwind CSS
- **Purpose**: Serve and manage an eBook library with OPDS protocol support

## Build Commands

### Backend (Go)

```bash
# Build backend (builds frontend too)
make backend

# Run all tests
make test

# Run single test
go test -tags "sqlite_omit_load_extension,fts5" -run TestName ./path/to/package

# Run tests with verbose output
go test -tags "sqlite_omit_load_extension,fts5" -v ./path/to/package

# Run server
air

# Initialize database
make init

# Scan library for books
make scan

# Clean database files
make clean
```

### Frontend (Vue 3 + Vite)

```bash
cd frontend
npm install          # Install dependencies
npm run dev         # Start dev server (http://localhost:5173)
npm run build        # Build for production
npm run preview      # Preview production build
```

### Docker

```bash
# Build production image
docker build -t bopds .

# Run development with hot reload
docker-compose -f docker-compose.dev.yml up

# Run production
docker-compose up
```

### Development Hot Reload

The `.air.toml` config enables hot reload for Go backend:

```bash
air -c .air.toml
```

## Code Style Guidelines

### Go Backend

#### Backend File Organization

- **app/**: HTTP handlers, routing, CLI entry point
- **repo/**: Database operations, SQLite queries, models
- **service/**: Business logic layer between handlers and repository
- **middleware/**: Request logging, recovery, request ID generation
- **scanner/**: Library scanning and book parsing (FB2 format from ZIP and 7z archives)
- **converter/**: Format conversion (FB2 → EPUB, MOBI) and archive extraction (ZIP, 7z)
- **validator/**: Input validation
- **book/**: Data models and types
- **config/**: Configuration loading (environment variables)
- **logger/**: Structured logging wrapper around slog

#### Backend Naming Conventions

- Package names: lowercase, single word (e.g., `repo`, `service`, `app`)
- Exported functions/variables: PascalCase (e.g., `GetAuthors`, `NewService`)
- Private functions/variables: camelCase (e.g., `getOrCreateAuthor`)
- Constants: PascalCase or UPPER_SNAKE_CASE
- Interfaces: typically named after functionality with `er` suffix (e.g., `Repository`)

#### Import Organization

```go
// 1. Standard library
import (
    "context"
    "database/sql"
    "fmt"
)

// 2. External packages (sorted alphabetically)
import (
    "github.com/htol/bopds/book"
    "github.com/htol/bopds/config"
    "github.com/google/uuid"
)

// 3. Blank imports only when needed (e.g., database drivers)
import _ "github.com/mattn/go-sqlite3"
```

#### Backend Error Handling

- Always wrap errors with context using `fmt.Errorf("%w", err)`
- Return errors from functions, don't panic unless truly unrecoverable
- Check errors immediately, don't defer error checks
- Use sentinel errors for expected conditions (e.g., `repo.ErrNotFound`)
- HTTP handlers: use `respondWithError()` or `respondWithValidationError()`

#### Testing Pattern

```go
package service

import (
    "context"
    "testing"
)

// Use init() for test setup if needed
func init() {
    logger.Init("info")
}

// Define mock repository inline
type mockRepository struct {
    data      []book.Author
    dataError error
}

func (m *mockRepository) GetAuthors() ([]book.Author, error) {
    if m.dataError != nil {
        return nil, m.dataError
    }
    return m.data, nil
}

// Test table-driven approach
func TestService_GetAuthors(t *testing.T) {
    tests := []struct {
        name        string
        data        []book.Author
        err         error
        expectError bool
    }{
        {
            name:        "success",
            data:        []book.Author{{FirstName: "Test", LastName: "Author"}},
            err:         nil,
            expectError: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mock := &mockRepository{data: tt.data, dataError: tt.err}
            svc := New(mock)

            ctx := context.Background()
            result, err := svc.GetAuthors(ctx)

            if tt.expectError && err == nil {
                t.Errorf("Expected error but got none")
            }
            // ... assertions
        })
    }
}
```

#### Database Operations

- Use prepared statements for all queries. ***NEVER*** use string interpolation in sql request.
- Always handle `sql.ErrNoRows` explicitly (return sentinel error)
- Use transactions for multi-step operations
- Defer `Close()` on prepared statements and rows
- Use context with queries: `QueryContext(ctx, query, args)`

#### Struct Tags

```go
type Book struct {
    BookID  int64  `json:"book_id" db:"book_id"`
    Title   string `json:"title"`
    Author  string `json:"author,omitempty"`
}
```

#### Context Usage

- Pass `context.Context` through service layer methods
- Use context for cancellation (long-running queries, external calls)
- Store request ID in context via middleware: `RequestIDKey`

### Frontend (Vue 3 + Composition API)

#### Frontend File Organization

```text
frontend/src/
├── components/
│   ├── base/         # Reusable UI components (BaseButton, BaseCard, etc.)
│   ├── domain/       # Domain-specific components (BookCard, AuthorCard, etc.)
│   ├── BooksView.vue
│   ├── AuthorsView.vue
│   ├── GenresView.vue
│   └── ...
├── api.js           # API client with fetch wrapper
├── stores/
│   └── libraryStore.js  # Reactive state management
├── main.js          # Entry point
└── App.vue          # Root component
```

#### Component Style

```vue
<template>
  <div class="wrapper-class">
    <ChildComponent
      v-model="value"
      @event="handleEvent"
    />
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import ChildComponent from './ChildComponent.vue'

// Props
const props = defineProps({
  modelValue: { type: String, default: '' },
  items: { type: Array, default: () => [] }
})

// Emits
const emit = defineEmits(['update:modelValue', 'event'])

// Reactive state
const value = ref(props.modelValue)

// Computed
const processed = computed(() => {
  return props.items.map(item => item.name)
})

// Watchers
watch(value, (newVal) => {
  emit('update:modelValue', newVal)
})

// Methods
const handleEvent = (data) => {
  // Handle event
}
</script>

<style scoped>
/* Component-specific styles */
</style>
```

#### Frontend Naming Conventions

- Components: PascalCase (e.g., `BookCard.vue`, `AuthorCard.vue`)
- Base components: Prefix with `Base` (e.g., `BaseButton.vue`)
- Domain components: Descriptive names (e.g., `SearchResultItem.vue`)
- Composables/functions: camelCase (e.g., `useLibraryStore`)
- Props/refs: camelCase
- Emits: camelCase or kebab-case
- CSS classes: kebab-case following Tailwind conventions

#### API Integration

Use the centralized API client in `api.js`:

```javascript
import { api, downloadBook } from './api'

// GET requests
const authors = await api.getAuthors('A')
const books = await api.getBooks('B')
const searchResults = await api.searchBooks('query', 20, 0)

// Download files
await downloadBook(bookId, 'fb2.zip')
```

#### State Management

- Use Vue's `reactive()` for global state (see `libraryStore.js`)
- No external state management library (no Pinia/Vuex)
- Export composable functions: `useLibraryStore()`
- Keep state minimal and focused

#### Styling (Tailwind CSS)

- Use Tailwind utility classes exclusively
- Custom colors defined in `tailwind.config.js`:
  - `bg-primary`, `bg-secondary` (backgrounds)
  - `accent-primary`, `accent-hover` (brand color: #0066FF)
  - `text-primary`, `text-muted` (typography)
  - `border-thin`, `border-thick` (borders)
- Minimal custom CSS; use `scoped` styles when needed
- Responsive design: use Tailwind breakpoints (`md:`, `lg:`)
- Dark mode: Not currently implemented

#### Frontend Error Handling

- API errors: try/catch with user-friendly messages
- Loading states: Show skeleton loaders or empty states
- Validation: Display inline error messages
- Use `EmptyState.vue` component for no-data states

#### Accessibility

- Use semantic HTML elements
- ARIA labels where needed
- Focus states for interactive elements
- Keyboard navigation support

## Configuration

### Environment Variables

Defined in `config/config.go` with defaults:

```bash
# Server
PORT=3001
LOG_LEVEL=info          # debug, info, warn, error

# Database
DB_PATH=./books.db
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=25
DB_CONN_MAX_LIFETIME=300

# Library
LIBRARY_PATH=./lib
```

### Frontend Environment

```bash
VITE_API_BASE_URL=    # Optional API base URL (default: same origin)
```

## Architecture Patterns

### Backend: Layered Architecture

1. **app/**: HTTP handlers, routing, CLI interface
2. **service/**: Business logic, orchestration
3. **repo/**: Data access, database operations
4. **middleware/**: Cross-cutting concerns (logging, recovery, CORS)

### Frontend: Component-Based

- Pages: `BooksView.vue`, `AuthorsView.vue`, `GenresView.vue`
- Shared components: `LibraryTabs.vue`, `AlphabetsFilter.vue`, `Paginator.vue`
- Base components: `BaseButton.vue`, `BaseCard.vue`, `BaseBadge.vue`, `BaseLoader.vue`
- Domain components: `BookCard.vue`, `AuthorCard.vue`, `SearchResultItem.vue`

### API Routes

```text
GET  /                          # Serve frontend (SPA)
GET  /api/authors               # Get all authors (legacy)
GET  /api/authors?startsWith=X   # Get authors by letter
GET  /api/authors/:id           # Get author by ID
GET  /api/authors/:id/books     # Get books by author
GET  /api/books                 # Get all books (legacy)
GET  /api/books?startsWith=X     # Get books by letter
GET  /api/books/:id/download     # Download book (format=fb2|fb2.zip|epub|mobi)
GET  /api/genres                # Get all genres
GET  /api/search                # Search books (q=, limit=, offset=)
GET  /health                    # Health check
```

## Development Workflow

1. Make changes to Go backend files
2. Hot reload via Air (automatic) or rebuild with `make build`
3. For frontend: Changes to `frontend/src/` trigger Vite hot reload
4. Run tests: `go test ./...` to verify backend
5. Build and test locally before committing

## Notes

- **Database**: SQLite with CGO required; build with `CGO_ENABLED=1`
- **Book format**: FB2 (FictionBook 2.0) stored in ZIP or 7z archives, converted on-demand to EPUB/MOBI
- **Archive support**: Both `.zip` and `.7z` archives are supported for book storage
- **Full-text search**: SQLite FTS5 virtual table for fast book search
- **CORS**: Enabled for all origins in development (`Access-Control-Allow-Origin: *`)
- **No test framework**: Uses standard Go testing only (no testify)
- **No frontend tests**: No test scripts defined in package.json
- **TypeScript**: Not used (plain JavaScript for frontend)
- **Linting**: No golangci-lint configuration present
