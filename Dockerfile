# builder
FROM golang:1.17-alpine AS builder
WORKDIR /build
# tzdata = in order to show correct local time respecting TZ env
RUN apk update && \
    apk add --no-cache git tzdata ca-certificates
COPY . .
RUN CGO_ENABLED=0 go build -o ./eqbc ./cmd/eqbc/main.go


# launcher
FROM scratch

ENV TZ=Europe/Stockholm

WORKDIR /app
COPY --from=builder /build/eqbc ./
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 2112
CMD [ "./eqbc" ]
