FROM gsoci.azurecr.io/giantswarm/alpine:3.19.1

RUN apk add --no-cache ca-certificates

ADD ./konfigure /konfigure

ENTRYPOINT ["/konfigure"]
