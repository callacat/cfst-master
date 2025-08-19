BINARY=controller
IMAGE=multi-net-controller:latest

.PHONY: build docker

build:
    go build -o $(BINARY) ./cmd/main.go

docker:
    docker build -t $(IMAGE) .

clean:
    rm -f $(BINARY)
