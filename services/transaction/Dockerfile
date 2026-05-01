FROM golang:1.23-alpine AS builder

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/transaction ./cmd/api

FROM alpine:3.20

RUN addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=builder /out/transaction /usr/local/bin/transaction
COPY docs ./docs
USER app
EXPOSE 8081 9091
CMD ["transaction"]

