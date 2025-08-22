FROM golang:1.24

WORKDIR /app

#dependencies
COPY go.mod go.sum ./
RUN go mod download

#build
COPY . .
RUN go build -o points ./cmd/server
RUN go build -o commit_points ./cmd/commit_points
RUN go build -o orders ./cmd/orders
RUN go build -o redeems ./cmd/redeems
RUN go build -o returns ./cmd/returns



