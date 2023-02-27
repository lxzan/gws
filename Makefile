test:
	go test -timeout 30s -run ^Test ./...

bench:
	go test -benchmem  -bench ^Benchmark github.com/lxzan/gws

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/gws-linux-amd64 github.com/lxzan/gws/examples/testsuite

cover:
	go test -coverprofile=./bin/cover.out --cover ./...
