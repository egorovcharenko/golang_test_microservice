FROM golang as builder
WORKDIR /go/src/github.com/egorovcharenko/golang_test_microservice
COPY service.go .
RUN go get github.com/gorilla/mux
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o service .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /go/src/github.com/egorovcharenko/golang_test_microservice .
CMD ["./service"]