FROM golang:1.23-alpine AS builder
WORKDIR /src
RUN apk add --no-cache gcc musl-dev sqlite-dev
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/ai-shortlink ./cmd/server

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates sqlite-libs wget && addgroup -S app && adduser -S app -G app && mkdir -p /app/data && chown -R app:app /app
COPY --from=builder /out/ai-shortlink /app/ai-shortlink
USER app
EXPOSE 8080
ENTRYPOINT ["/app/ai-shortlink"]
