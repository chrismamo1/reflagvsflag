FROM golang:1.8
EXPOSE 80
WORKDIR /go/src/github.com/chrismamo1
ADD . /go/src/github.com/chrismamo1/reflagvsflag

RUN go get github.com/gorilla/mux && \
        go get github.com/lib/pq && \
        go get github.com/aws/aws-sdk-go

WORKDIR /go/src/github.com/chrismamo1/reflagvsflag
CMD ls && pwd && go run ./main.go
