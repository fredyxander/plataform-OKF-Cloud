FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

ARG SERVICE

RUN CGO_ENABLED=0 GOOS=linux \
    go build -o /bin/app ./cmd/${SERVICE}


FROM alpine:3.21

WORKDIR /app

COPY --from=builder /bin/app /app/app

CMD ["/app/app"]