FROM docker.io/library/golang:1.16 as build

WORKDIR /go/src/app
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

FROM docker.io/library/ubuntu:latest
COPY --from=build /go/src/app/imap-tags /usr/bin/imap-tags

CMD ["imap-tags"]
