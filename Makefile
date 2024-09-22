test:
	go test ./...

bench:
	go test -benchmem -run=^$$ -bench . github.com/lxzan/gws

cover:
	go test -coverprofile=./bin/cover.out --cover ./...

clean:
	rm -rf ./bin/*
