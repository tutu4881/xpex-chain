FROM golang:1.26-alpine AS build

RUN apk add --no-cache make git libc-dev bash gcc linux-headers

WORKDIR /app

COPY . .

# go.mod pins cosmos-sdk v0.54.4 / CometBFT v0.39.4, but the committed go.sum and the
# indirect requirements still come from the earlier pre-release. `go mod tidy` resolves
# and verifies them (against sum.golang.org) inside the build.
# TODO: commit the tidied go.mod/go.sum once a build machine with module access is available.
RUN go mod tidy

RUN go build -o /xpexd ./exampled


FROM alpine:3.22

RUN apk add --no-cache bash curl jq sed python3

COPY --from=build /xpexd /usr/local/bin/xpexd

RUN mkdir -p /data/.exampleapp/config /data/.exampleapp/data /opt/xpex

COPY railway-config/config/genesis.json /opt/xpex/genesis.json
COPY railway-config/config/genesis.staging.json /opt/xpex/genesis.staging.json
COPY railway-start.sh /usr/bin/railway-start.sh

RUN chmod +x /usr/bin/railway-start.sh

EXPOSE 26656
EXPOSE 26657
EXPOSE 1317
EXPOSE 9090

WORKDIR /root

ENTRYPOINT ["/usr/bin/railway-start.sh"]
