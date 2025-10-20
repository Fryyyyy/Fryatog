# Build stage
FROM golang:1 AS builder
WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 go build -v -o /fryatog

# Final stage
FROM alpine:latest
WORKDIR /go/src/app
# config.json is expected to be mounted under /go/src/app/config.json
COPY short_names.json ./ 
COPY --from=builder /fryatog ./fryatog
CMD ["./fryatog"]
