.PHONY: test bench vet fmt run

test:
	go test ./...

bench:
	go test -bench=. -benchmem ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

run:
	go run ./cmd/api