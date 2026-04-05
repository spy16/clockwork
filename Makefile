VERSION:=$(shell git describe --abbrev=0 --tags)
COMMIT:=$(shell git rev-list -1 HEAD)
BUILT_ON:=$(shell date +'%Y-%m-%d_%T')
OUT_PATH="./out"

all: update-swagger protogen clean fmt test build

setup:
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/vektra/mockery/v2@v2.35.4

install:
	@echo "Installing..."
	@go install ./cmd/clockwork

build: update-swagger
	@echo "Building..."
	@mkdir -p ${OUT_PATH}
	@go build -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuiltOn=$(BUILT_ON)" -o ${OUT_PATH}/clockwork ./cmd/clockwork

build-linux: generate
	@echo "Building..."
	@mkdir -p ${OUT_PATH}
	@GOOS=linux go build -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuiltOn=$(BUILT_ON)" -o ${OUT_PATH}/clockwork ./cmd/clockwork

fmt:
	@echo "Formatting..."
	@goimports -l -w ./

clean:
	@echo "Cleaning up..."
	@go mod tidy -v
	@rm -rf ${OUT_PATH}
	@mkdir -p ${OUT_PATH}

protogen:
	@echo "Generating protobuf message definitions..."
	@protoc --go_out=./api/ ./api/proto/*.proto

generate:
	@echo "Running go generate..."
	@go generate ./...

test:
	@echo "Running unit tests..."
	@go test -race -cover -coverprofile=coverage.out ./... | grep -v "no test files"
	@go tool cover -html=coverage.out -o ${OUT_PATH}/coverage.html
	@go tool cover -func=coverage.out | grep -i total:

test-all:
	@echo "Running unit tests..."
	@go test -race -cover -tags integration -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o ${OUT_PATH}/coverage.html
	@go tool cover -func=coverage.out | grep -i total:

test-verbose:
	@echo "Running unit tests..."
	@go test -race -cover -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o ${OUT_PATH}/coverage.html
	@go tool cover -func=coverage.out | grep -i total:

benchmark:
	@echo "Running benchmarks..."
	@go test -benchmem -run="none" -bench="Benchmark.*" -v ./...

server/swagger:
	@echo "Downloading Swagger UI..."
	@mkdir -p /tmp/swagger
	curl -L -o /tmp/swagger/swagger.tar.gz https://github.com/swagger-api/swagger-ui/archive/v3.20.5.tar.gz
	@tar -C /tmp/swagger -xzf /tmp/swagger/swagger.tar.gz
	@mkdir -p ./server/swagger
	@mv /tmp/swagger/swagger-ui-3.20.5/dist/* ./server/swagger/

update-swagger: server/swagger
	@echo "Updating swagger.yml..."
	@cp ./api/swagger.yml ./server/swagger/
	@sed 's/\s*url\:.*,/url: ".\/swagger.yml",/' server/swagger/index.html > temp.index
	@mv temp.index server/swagger/index.html
