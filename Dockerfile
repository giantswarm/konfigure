FROM gsoci.azurecr.io/giantswarm/alpine:3.24.1

ARG TARGETARCH

# ca-certificates is intentionally unpinned: pinning would freeze the Mozilla trust store,
# preventing revoked or removed CAs from being dropped on rebuild.
# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates

COPY ./konfigure-linux-${TARGETARCH} /konfigure

ENTRYPOINT ["/konfigure"]
