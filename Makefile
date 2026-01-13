.PHONY: frontend build test init scan serve clean

all: build

clean:
	rm -f books.db books.db.backup books.db-wal books.db-shm

frontend:
	cd frontend; npm run build

build: frontend
	CGO_ENABLED=1 go build -tags "sqlite_omit_load_extension,fts5"

test:
	go test -tags "sqlite_omit_load_extension,fts5" ./...

init: build clean
	./bopds init; ls -alsh books.db

scan: build init
	bash -c "time ./bopds scan"
	ls -alsh books.db

serve: build
	./bopds serve

env:
	go install github.com/air-verse/air@latest
