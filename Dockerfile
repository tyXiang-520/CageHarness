# Multi-stage build
# Stage 1: Build
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /build/harness ./cmd/harness

# Stage 2: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/harness .
COPY config.example.yaml /app/config.example.yaml
COPY web/ /app/web/
EXPOSE 8080
ENTRYPOINT ["./harness"]