run:
	go run cmd/main.go

build:
	go mod tidy
	go build cmd/main/go -o tq.exe

deps:
	go mod tidy

clean:
	rm tq.exe