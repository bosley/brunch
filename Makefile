all:
	go build -o bru ./cmd/bru

clean:
	rm -f bru
