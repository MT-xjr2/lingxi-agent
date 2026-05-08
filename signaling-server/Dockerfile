FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o signaling-server .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/signaling-server /usr/local/bin/
EXPOSE 9090
CMD ["signaling-server"]
