FROM golang:1.23.4-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN go build -o nori ./cmd/nori

COPY --from=builder /app/nori /usr/local/bin/nori

CMD ["nori"]
