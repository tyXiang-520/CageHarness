# Multi-stage build
# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod ./
# No go.sum needed — zero external dependencies
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /build/harness ./cmd/harness

# Stage 2: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/harness .
EXPOSE 8080
ENV PORT=8080
ENTRYPOINT ["./harness", "serve"]