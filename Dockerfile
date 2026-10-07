FROM golang:1.27 AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y gcc libc6-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o server .

FROM debian:bookworm-slim

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/web ./web
COPY --from=builder /app/presets ./presets

CMD ["./server"]