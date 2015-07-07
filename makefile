prepare:
	@echo "*** Create bin & pkg dirs, if not exists..."
	@mkdir -p bin
	@mkdir -p pkg

get:
	@echo "*** Resolve dependencies..."
	@go get -v github.com/gorilla/mux
	@go get -v github.com/stretchr/testify

test:
	@echo "*** Run tests..."
	go test -v ./src/memdb/...
	go test -v ./src/rest/...

test-race:
	@echo "*** Run tests with race condition..."
	@go test --race -v ./src/memdb/...
	@go test --race -v ./src/rest/...

test-cover:
	@go test -covermode=count -coverprofile=/tmp/coverage_memdb.out ./src/memdb/...
	@go test -covermode=count -coverprofile=/tmp/coverage_rest.out ./src/rest/...

	@rm -f /tmp/memdb_coverage.out
	@echo "mode: count" > /tmp/memdb_coverage.out
	@cat /tmp/coverage_memdb.out | tail -n +2 >> /tmp/memdb_coverage.out
	@rm /tmp/coverage_memdb.out
	@cat /tmp/coverage_rest.out | tail -n +2  >> /tmp/memdb_coverage.out
	@rm /tmp/coverage_rest.out

	@go tool cover -html=/tmp/memdb_coverage.out

build:
	@echo "*** Build project..."
	@go build -v -o bin/memdb src/main.go

build-race:
	@echo "*** Build project with race condition..."
	@go build --race -v -o bin/memdb-race src/main.go

clean-bin:
	@echo "*** Clean up bin/ directory..."
	@rm -rf bin/*

clean-pkg:
	@echo "*** Clean up pkg/ directory..."
	@rm -rf pkg/*

clean: clean-bin clean-pkg

all: prepare clean get test-race build-race
	@echo "*** Done!"
