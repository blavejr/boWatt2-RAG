# build stage
FROM golang:1.23.1-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bowattAI .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/bowattAI .

RUN mkdir -p /app/uploads /app/evaluation

EXPOSE 8080

CMD ["./bowattAI"]

