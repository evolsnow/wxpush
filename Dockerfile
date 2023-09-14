FROM golang:alpine as builder
WORKDIR /root/
COPY main.go go.mod go.sum ./
RUN go build -o wxpush .

FROM alpine:latest as prod
WORKDIR /root/
COPY --from=builder /root/wxpush .

CMD ["./wxpush"]