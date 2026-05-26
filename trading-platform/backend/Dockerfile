FROM golang:1.25 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-arm64} go build -o /out/trading-backend ./cmd/main.go

FROM debian:bookworm-slim

WORKDIR /app
COPY --from=builder /out/trading-backend ./trading-backend

EXPOSE 3000

CMD ["./trading-backend"]