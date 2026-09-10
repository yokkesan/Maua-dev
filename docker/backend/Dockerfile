FROM golang:1.26-alpine

WORKDIR /app

RUN apk add --no-cache git

COPY backend/go.mod backend/go.sum* ./

RUN go mod download

COPY backend/ ./

EXPOSE 8080

CMD ["go", "run", "."]