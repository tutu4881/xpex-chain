FROM golang:1.25-alpine AS build

RUN apk add --no-cache make git libc-dev bash gcc linux-headers

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /xpexd ./exampled


FROM alpine:3.22

RUN apk add --no-cache bash curl jq sed python3

COPY --from=build /xpexd /usr/local/bin/xpexd

RUN mkdir -p /data/.exampleapp/config /data/.exampleapp/data /opt/xpex

COPY railway-config/config/genesis.json /opt/xpex/genesis.json
COPY railway-start.sh /usr/bin/railway-start.sh

RUN chmod +x /usr/bin/railway-start.sh

EXPOSE 26656
EXPOSE 26657
EXPOSE 1317
EXPOSE 9090

WORKDIR /root

ENTRYPOINT ["/usr/bin/railway-start.sh"]
