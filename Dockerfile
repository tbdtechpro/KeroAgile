# Stage 1: build React web UI
FROM node:20-alpine AS web-builder
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: build Go binary (with embedded web assets)
FROM golang:1.25-alpine AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /KeroAgile ./cmd/keroagile/

# Stage 3: minimal runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=go-builder /KeroAgile /usr/local/bin/KeroAgile
ENV KEROAGILE_DATA_DIR=/data
VOLUME ["/data"]
EXPOSE 7432
ENTRYPOINT ["KeroAgile"]
CMD ["mcp"]
