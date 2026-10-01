FROM golang:1.25-alpine AS builder

WORKDIR /build

RUN apk add --no-cache ca-certificates build-base git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -o manager \
    -ldflags="-s -w" \
    -trimpath \
    ./cmd/main.go

# receipt-writer image. Build with: docker build --target receipt-writer .
FROM builder AS receipt-writer-build
RUN CGO_ENABLED=0 GOOS=linux go build -o receipt-writer \
    -ldflags="-s -w" \
    -trimpath \
    ./cmd/receipt-writer

FROM gcr.io/distroless/static:nonroot AS receipt-writer
COPY --from=receipt-writer-build /build/receipt-writer /receipt-writer
USER nonroot:nonroot
ENTRYPOINT ["/receipt-writer"]

# Default target: the operator manager. Keep this stage last.
FROM gcr.io/distroless/static:nonroot

LABEL maintainer="operator-maintainers@example.com"
LABEL description="Kubernetes operator for agentic workloads"

COPY --from=builder /build/manager /manager

USER nonroot:nonroot

ENTRYPOINT ["/manager"]
