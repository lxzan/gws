test:
	go test -timeout 30s -run ^Test github.com/lxzan/gws

bench:
	go test -benchmem  -bench ^Benchmark github.com/lxzan/gws
