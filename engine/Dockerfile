FROM golang:1.24

WORKDIR /app

# dependencies
COPY go.mod go.sum ./
RUN go mod download

# build
COPY . .
RUN go build -o engine ./cmd


CMD ["./engine"]
