ODULE = $(shell go list -m)
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || echo "1.0.0")
PACKAGES := $(shell go list ./... | grep -v -e server -e test -e middleware -e entity -e constants -e mocks -e errors | sort -r )
LDFLAGS := -ldflags "-X main.Version=${VERSION}"

CONFIG_FILE ?= ./config/local.yml
APP_DSN ?= $(shell sed -n 's/^dsn:[[:space:]]*"\(.*\)"/\1/p' $(CONFIG_FILE))
MIGRATE := migrate -path migrations -database "$(APP_DSN)"
DOCKER_REPOSITORY := hinccvi/server
MOCKERY := mockery --name=Repository -r --output=./internal/mocks

PID_FILE := './.pid'
FSWATCH_FILE := './fswatch.cfg'


.PHONY: help
help: ## help information about make commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: run
run: ## run the Go project
	go run ${LDFLAGS} cmd/server/main.go

.PHONY: build
build:  ## build the Go project
	go build ${LDFLAGS} -o myapp cmd/server/main.go

.PHONY: clean
clean: ## remove temporary files
	rm -rf myapp
