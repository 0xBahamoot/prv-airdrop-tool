FROM golang:1.16.2-stretch AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN cd /app/nftdrop \
    && go build -tags=jsoniter -ldflags "-linkmode external -extldflags -static" -o nftdrop-service

FROM alpine

WORKDIR /app

COPY --from=build /app/nftdrop/nftdrop-service /app/nftdrop-service

CMD [ "./airdrop-service" ]
