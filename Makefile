.PHONY: test bench vet fmt run

PACKAGES := ./cmd/... ./internal/... ./examples/...

test:
	go test $(PACKAGES)

bench:
	cd internal/limiter && go test -run=^$$ -bench=. -benchmem
	cd internal/middleware && go test -run=^$$ -bench=. -benchmem

vet:
	go vet $(PACKAGES)

fmt:
	go fmt $(PACKAGES)

run:
	go run ./cmd/api