FROM golang:1.26.2-alpine3.23 AS build
ARG appName
ARG appVersion
ARG vcsRef

WORKDIR /build

RUN apk add upx

COPY ./go.mod go.sum ./
RUN go mod download

COPY ./cmd/ ./cmd
COPY ./internal/ ./internal

RUN go build -ldflags "\
    -s -w \
    -X main.AppName=${appName} \
    -X main.AppVersion=${appVersion} \
    -X main.BuildDate=$(date -u +'%Y-%m-%dT%H:%M:%SZ')\
    " -o dmc-dataloader-api ./cmd/dmc-dataloader-api

RUN upx -9 dmc-dataloader-api

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=build /build/dmc-dataloader-api /app/

EXPOSE 3000

ENTRYPOINT ["/app/dmc-dataloader-api"]
CMD []
