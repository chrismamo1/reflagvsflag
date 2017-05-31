FROM golang:1.8
EXPOSE 3456
RUN go get github.com/gorilla/mux
RUN go get github.com/lib/pq
RUN go get github.com/chrismamo1/reflagvsflag
RUN pwd && ls
CMD go get github.com/chrismamo1/reflagvsflag && cd /go/src/github.com/chrismamo1/reflagvsflag && ls && pwd && go run ./main.go
