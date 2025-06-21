all: build

build:
	go build

test:
	go test ./...

init: build
	rm -f ./books.db
	./bopds init
	ls -alsh books.db

scan: build init
	bash -c "time ./bopds scan"
	ls -alsh books.db

serve: build
	./bopds serve
