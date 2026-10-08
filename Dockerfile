# The deployment lock supplies an exact builder digest.
ARG GO_IMAGE=golang:1.27.1-bookworm@sha256:8d48e12ec56735e9358640898b9d9b9fcca110612ed8a5567438c0a1baa24e66
FROM ${GO_IMAGE} AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/forge-api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/forge-migrate ./cmd/forge-migrate && \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/forge-environment-controller ./cmd/forge-environment-controller

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/ /
USER 65532:65532
ENTRYPOINT ["/forge-api"]
