all:
	go build -o brucli ./cmd/brucli
clean:
	rm -f brucli
