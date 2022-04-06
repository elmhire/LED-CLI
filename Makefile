## LED-CLI Makefile
run_installed: install
	go run

install: vet
	go install

run: vet
	go run main.go

build: vet
	go build -o LED-CLI ./main.go

build_win: vet
	env GOOS=windows GOARCH=amd64 go build -o LED-CLI.exe ./main.go

vet: fmt
	go vet ./...

lint: fmt
	golint ./...

fmt:
	go fmt ./...





.PHONY: fmt lint vet build run
