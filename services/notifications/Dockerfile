FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/notifications-consumer ./cmd/consumer
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/notifications-worker ./cmd/worker

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/notifications-consumer /app/notifications-consumer
COPY --from=build /out/notifications-worker /app/notifications-worker
CMD ["/app/notifications-consumer"]