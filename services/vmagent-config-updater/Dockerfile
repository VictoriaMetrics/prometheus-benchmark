FROM golang:1.23.4-alpine AS builder

WORKDIR /config-updater

COPY . .

RUN CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -trimpath -buildvcs=false \
    -ldflags "-s -w -extldflags '-static'" \
    -tags 'netgo osusergo nethttpomithttp2' \
    -o bin/config-updater .

FROM scratch

EXPOSE 8436

ENTRYPOINT ["/config-updater"]
COPY --from=builder /config-updater/bin/config-updater ./config-updater
