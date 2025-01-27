all:
	go build -o brucli ./cmd/brucli
test:
	go test -v -count=1 ./...
clean:
	rm -f brucli
