MODULE = $(shell go list -m)
GIT_COMMIT=$(shell git rev-parse --short HEAD)

.PHONY: generate build test lint build-docker compose compose-down migrate

generate:
	go generate ./...

build: # build a server
	go build -a -ldflags "-s -X github.com/chuihairu/croupier/internal/version.GitCommit=$(GIT_COMMIT)" -o server github.com/chuihairu/croupier/cmd/server

debug-build:
	go build -gcflags "all=-N -l" -a -ldflags "-compressdwarf=false -s -X github.com/chuihairu/croupier/internal/version.GitCommit=$(GIT_COMMIT)" -o server github.com/chuihairu/croupier/cmd/server

test:
	go clean -testcache
	go test ./... -v

lint:
	gofmt -l .

docker-build: # build docker image
	docker build --progress=plain --no-cache -f build/package/Dockerfile -t cuihairu/croupier --build-arg GITCOMMIT=$(GIT_COMMIT) .

compose.%:
	$(eval CMD = ${subst compose.,,$(@)})
	tools/script/compose.sh $(CMD)

migrate:
	docker run --rm -v migrations:/migrations --network host migrate/migrate -path=/migrations/ \
	-database mysql://root:password@localhost:3306/local_db?charset=utf8&parseTime=True&multiStatements=true up 2
