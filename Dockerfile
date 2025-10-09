FROM gsoci.azurecr.io/giantswarm/alpine:3.22.2

RUN apk add --no-cache ca-certificates

ADD ./konfigure /konfigure

ENTRYPOINT ["/konfigure"]
