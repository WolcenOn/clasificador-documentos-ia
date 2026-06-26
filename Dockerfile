FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY backend/go.mod ./
RUN go mod download

COPY backend/ ./
RUN go build -o server ./cmd/server

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/server ./server

EXPOSE 8080
CMD ["./server"]
