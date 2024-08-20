build:
	go build -o /usr/bin/nori main.go
dev-test:
	go test -v ./...