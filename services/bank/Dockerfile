FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/bank-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/outbox-relay ./cmd/outbox-relay

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/bank-api /app/bank-api
COPY --from=build /out/outbox-relay /app/outbox-relay
CMD ["/app/bank-api"]