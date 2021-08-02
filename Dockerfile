FROM golang:1.16.2-stretch AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -tags=jsoniter -ldflags "-linkmode external -extldflags -static" -o airdrop-service

FROM alpine

WORKDIR /app

COPY --from=build /app/airdrop-service /app/airdrop-service

CMD [ "./airdrop-service" ]
