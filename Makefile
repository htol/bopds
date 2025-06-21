.PHONY: frontend build test init scan serve

all: build

frontend:
	cd frontend; npm run build

build: frontend
	go build

test:
	go test ./...

init: build
	rm -f ./books.db; ./bopds init;	ls -alsh books.db

scan: build init
	bash -c "time ./bopds scan"
	ls -alsh books.db

serve: build 
	./bopds serve
