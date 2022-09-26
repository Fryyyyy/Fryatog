FROM golang:1
WORKDIR /go/src/app

COPY . .

RUN go install -v

CMD ["fryatog"]
