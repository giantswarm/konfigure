FROM gsoci.azurecr.io/giantswarm/alpine:3.21.3

RUN apk add --no-cache ca-certificates

ADD ./konfigure /konfigure

ENTRYPOINT ["/konfigure"]
