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

all: clean get test-race build-race
	@echo "*** Done!"
