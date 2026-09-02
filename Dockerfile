ARG GO_VERSION=1.26
FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X go-web-template/internal/buildinfo.Version=${VERSION} -X go-web-template/internal/buildinfo.Commit=${COMMIT} -X go-web-template/internal/buildinfo.BuildTime=${BUILD_TIME}" \
    -o /out/server ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S -G app app
COPY --from=build /out/server /usr/local/bin/server
USER app
EXPOSE 8080
ENTRYPOINT ["server"]
