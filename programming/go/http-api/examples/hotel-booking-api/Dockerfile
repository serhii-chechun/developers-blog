FROM golang:1.27-alpine AS builder
WORKDIR /build

COPY go.mod go.sum ./
COPY cmd cmd
COPY internal internal

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/booking-api ./cmd/booking-api/

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /

COPY --from=builder /tmp/booking-api .
EXPOSE 8080
ENTRYPOINT [ "./booking-api" ]