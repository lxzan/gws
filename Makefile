GO?=go
PACKAGES=           $(shell $(GO) list ./... | grep -v 'examples')
PACKAGE_DIRS=       $(shell $(GO) list -f '{{ .Dir }}' ./... | grep -v 'examples')
.PHONY: all vet lint

all: vet lint test bench cover autobahn

vet:
	$(GO) vet $(PACKAGES)

lint:
	golangci-lint run $(PACKAGE_DIRS)

test:
	go test -timeout 30s -run ^Test ./...

bench:
	go test -benchmem  -bench ^Benchmark github.com/lxzan/gws

cover:
	go test -coverprofile=./bin/cover.out --cover ./...

autobahn: clean
	mkdir -p ./autobahn/bin
	$(GO) build -o ./autobahn/bin/autobahn_server ./autobahn/server
	$(GO) build -o ./autobahn/bin/autobahn_reporter ./autobahn/reporter
	./autobahn/script/run_autobahn.sh

clean:
	rm -rf ./autobahn/bin
	rm -rf ./autobahn/reports

.PHONY: all vet lint test autobahn clean

