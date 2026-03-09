FROM golang:1.25-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/tasks-service ./cmd/tasks-service

FROM alpine:3.21
WORKDIR /app
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /out/tasks-service /app/tasks-service

EXPOSE 8080
ENTRYPOINT ["/app/tasks-service"]
