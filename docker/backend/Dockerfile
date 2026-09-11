FROM golang:1.26-alpine

WORKDIR /app

RUN apk add --no-cache git

COPY src/backend/go.mod src/backend/go.sum* ./

RUN go mod download

COPY src/backend/ ./

EXPOSE 8080

CMD ["go", "run", "."]