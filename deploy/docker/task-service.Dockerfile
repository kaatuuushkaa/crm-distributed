FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /build/task-service \
    ./cmd/task-service

FROM gcr.io/distroless/static-debian12

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /build/task-service /task-service

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/task-service"]