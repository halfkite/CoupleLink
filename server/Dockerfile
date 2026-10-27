FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /couplelink ./cmd/couplelink

FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 couplelink
COPY --from=build /couplelink /usr/local/bin/couplelink
USER couplelink
ENV COUPLELINK_LISTEN_ADDR=0.0.0.0:8080
EXPOSE 8080
ENTRYPOINT ["couplelink"]
