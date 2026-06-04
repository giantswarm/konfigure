FROM gsoci.azurecr.io/giantswarm/alpine:3.23.3

ARG TARGETARCH

# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates

COPY ./konfigure-linux-${TARGETARCH} /konfigure

ENTRYPOINT ["/konfigure"]
