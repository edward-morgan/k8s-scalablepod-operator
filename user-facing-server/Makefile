name = user-facing-server
tag = 0.1

all: fmt vet docker-build

fmt:
	go fmt ./...

vet: 
	go vet ./...

docker-build:
	docker build . -t $(name):$(tag)
