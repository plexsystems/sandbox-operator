FROM golang:1.13-alpine AS builder
WORKDIR /operator

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN GOOS=linux GOARCH=amd64 go build -o sandbox-operator main.go

FROM alpine:3.11.2
ENV OPERATOR=/usr/local/bin/sandbox-operator \
    USER_UID=1001 \
    USER_NAME=sandbox-operator

COPY --from=builder /operator/sandbox-operator ${OPERATOR}
COPY scripts/ /usr/local/bin

RUN chmod +x /usr/local/bin/user_setup
RUN chmod +x /usr/local/bin/entrypoint
RUN chmod +x /usr/local/bin/sandbox-operator

RUN /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
