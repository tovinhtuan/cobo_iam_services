FROM golang:1.23-alpine

WORKDIR /app

RUN apk add --no-cache bash ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Default command is API in dev mode; docker-compose can override.
CMD ["go", "run", "./cmd/api"]
