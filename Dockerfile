FROM golang:alpine

ENV GOPROXY="https://goproxy.io,direct"
##https://proxy.inodes.org
COPY ./ /usr/src/app/challenge

WORKDIR /usr/src/app/challenge/cmd

COPY ./conf/dev-conf.yaml ./conf/

COPY ./pkg ./pkg
COPY ./go.mod ./
COPY ./go.sum ./
RUN go mod download

COPY ./cmd/*.go  ./
RUN go build -o /challenge
EXPOSE 1402
CMD ["/challenge"]