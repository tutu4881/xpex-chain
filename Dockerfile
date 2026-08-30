# Build stage
FROM golang:1.25-alpine AS build

RUN apk add --no-cache make git libc-dev bash gcc linux-headers

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /xpexd ./exampled

# Runtime stage
FROM alpine:3.22

RUN apk add --no-cache bash curl jq

COPY --from=build /xpexd /usr/local/bin/xpexd

WORKDIR /root

# XPEX production configuration
COPY railway-config/config/genesis.json /root/.exampleapp/config/genesis.json
COPY railway-start.sh /usr/bin/railway-start.sh

RUN chmod +x /usr/bin/railway-start.sh

EXPOSE 26656 26657 1317 9090

ENTRYPOINT ["/usr/bin/railway-start.sh"]
