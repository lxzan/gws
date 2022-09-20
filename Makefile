test:
	go test -count 1 -timeout 30s -run ^Test ./...

bench:
	go test -benchmem  -bench ^Benchmark github.com/lxzan/gws
