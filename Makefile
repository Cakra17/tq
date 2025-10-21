run-scheduler:
	go run cmd/scheduler/main.go

run-worker:
	go run cmd/worker/main.go

build:
	go build -o bin/scheduler ./cmd/scheduler
	go build -o bin/worker ./cmd/worker

deps:
	go mod tidy

clean:
	rm -r ./bin/
