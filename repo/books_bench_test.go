package repo

import (
	"os"
	"testing"

	"github.com/htol/bopds/book"
)

func BenchmarkGetOrCreateAuthor_Existing(b *testing.B) {
	os.Remove("./bench_existing.db")
	db := GetStorage("bench_existing.db")
	defer db.Close()
	defer os.Remove("./bench_existing.db")

	author := book.Author{
		FirstName:  "John",
		MiddleName: "Quincy",
		LastName:   "Adams",
	}
	authorIDs, err := getOrCreateAuthorHelper(db, []book.Author{author})
	if err != nil {
		b.Fatalf("Failed to create author: %v", err)
	}
	if len(authorIDs) != 1 {
		b.Fatalf("Expected 1 author ID, got %d", len(authorIDs))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := getOrCreateAuthorHelper(db, []book.Author{author})
		if err != nil {
			b.Fatalf("getOrCreateAuthor failed: %v", err)
		}
	}
}

func BenchmarkGetOrCreateAuthor_Mixed(b *testing.B) {
	os.Remove("./bench_mixed.db")
	db := GetStorage("bench_mixed.db")
	defer db.Close()
	defer os.Remove("./bench_mixed.db")

	existingAuthors := []book.Author{
		{FirstName: "Author", MiddleName: "One", LastName: "Test"},
		{FirstName: "Author", MiddleName: "Two", LastName: "Test"},
		{FirstName: "Author", MiddleName: "Three", LastName: "Test"},
	}
	for _, author := range existingAuthors {
		_, err := getOrCreateAuthorHelper(db, []book.Author{author})
		if err != nil {
			b.Fatalf("Failed to create initial author: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			_, err := getOrCreateAuthorHelper(db, []book.Author{existingAuthors[i%3]})
			if err != nil {
				b.Fatalf("getOrCreateAuthor failed: %v", err)
			}
		} else {
			newAuthor := book.Author{
				FirstName:  "New",
				MiddleName: "Author",
				LastName:   string(rune(i)),
			}
			_, err := getOrCreateAuthorHelper(db, []book.Author{newAuthor})
			if err != nil {
				b.Fatalf("getOrCreateAuthor failed: %v", err)
			}
		}
	}
}

func BenchmarkGetOrCreateAuthor_New(b *testing.B) {
	os.Remove("./bench_new.db")
	db := GetStorage("bench_new.db")
	defer db.Close()
	defer os.Remove("./bench_new.db")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		author := book.Author{
			FirstName:  "New",
			MiddleName: "Author",
			LastName:   string(rune(i % 1000)),
		}
		_, err := getOrCreateAuthorHelper(db, []book.Author{author})
		if err != nil {
			b.Fatalf("getOrCreateAuthor failed: %v", err)
		}
	}
}
