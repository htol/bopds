.PHONY: frontend build test init scan serve clean

all: build

build: frontend backend

clean:
	rm -f books.db books.db.backup books.db-wal books.db-shm

frontend:
	cd frontend; npm run build

backend:
	CGO_ENABLED=1 go build -tags "sqlite_omit_load_extension,fts5"

docker-prod:
	docker build -t bopds:production .

docker-dev:
	docker build -t bopds:development .

dev:
	tmux new-session -d -s bopds-dev 'cd frontend && npm run dev'
	tmux split-window -v -t bopds-dev 'air'
	tmux attach-session -t bopds-dev

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
	go install golang.org/x/tools/gopls@latest

update-deps:
	GOPROXY=direct go get github.com/htol/fb2c@main
	go mod tidy
