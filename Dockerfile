FROM docker.io/library/golang:1.17 as build

WORKDIR /go/src/app
COPY . .

RUN go get -d -v ./...
RUN go build ./cmd/imaptags

FROM docker.io/library/ubuntu:latest
COPY --from=build /go/src/app/imaptags /usr/bin/imaptags

CMD ["imaptags"]
