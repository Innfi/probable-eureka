# Stage 1: build
FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o probable-eureka .

# Stage 2: installer image
FROM alpine:3.19

COPY --from=builder /src/probable-eureka /probable-eureka

CMD ["cp", "/probable-eureka", "/opt/cni/bin/probable-eureka"]
