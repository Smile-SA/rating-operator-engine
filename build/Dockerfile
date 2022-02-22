FROM golang:alpine AS builder

WORKDIR $GOPATH/src/github.com/rating-operator-engine

COPY . .

# Install the dependencies & packages
RUN go mod vendor

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-w -s -extldflags "-static"' -o /go/bin/rating-operator-engine -mod=vendor main.go

FROM scratch

COPY --from=builder /go/bin/rating-operator-engine /go/bin/rating-operator-engine

ENTRYPOINT ["/go/bin/rating-operator-engine"]
