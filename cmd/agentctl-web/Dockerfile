FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /agentctl-web ./cmd/agentctl-web/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /agentctl-web /agentctl-web
EXPOSE 8090
USER nonroot:nonroot
ENTRYPOINT ["/agentctl-web"]
