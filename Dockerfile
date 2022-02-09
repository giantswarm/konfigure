FROM alpine:3.15.0

RUN apk add --no-cache ca-certificates

ADD ./konfigure /konfigure

ENTRYPOINT ["/konfigure"]
