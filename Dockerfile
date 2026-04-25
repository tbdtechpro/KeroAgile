FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /KeroAgile ./cmd/keroagile/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /KeroAgile /usr/local/bin/KeroAgile
ENV KEROAGILE_DATA_DIR=/data
VOLUME ["/data"]
EXPOSE 7432
ENTRYPOINT ["KeroAgile"]
CMD ["mcp"]
