##
## Build
##
FROM golang:1.17-bullseye AS build

WORKDIR /go/src/app

ADD . /go/src/app

RUN go get -d -v ./...

RUN go build -o /go/bin/gateway

##
## Deploy
##
FROM gcr.io/distroless/base-debian11

WORKDIR /octopus-gateway

COPY --from=build /go/bin/gateway /octopus-gateway/gateway

EXPOSE 80

ENTRYPOINT ["/octopus-gateway/gateway"]