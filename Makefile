test:
	go test -count 1 -timeout 30s -run ^Test ./...

bench:
	go test -benchmem  -bench ^Benchmark github.com/lxzan/gws

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/gws-server-linux-amd64 github.com/lxzan/gws/examples/testsuite

testsuite-server:
	go run github.com/lxzan/gws/examples/testsuite
