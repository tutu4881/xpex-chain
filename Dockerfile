FROM golang:1.25-alpine AS build

RUN apk add --no-cache git make gcc libc-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /xpexd ./exampled

FROM alpine:3.22

RUN apk add --no-cache bash curl jq

COPY --from=build /xpexd /usr/local/bin/xpexd

WORKDIR /root

EXPOSE 26656 26657 1317 9090

CMD ["xpexd", "start", "--minimum-gas-prices", "0.001uxpx"]
