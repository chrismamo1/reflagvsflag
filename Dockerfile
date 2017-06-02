FROM golang:1.8
EXPOSE 80
RUN go get github.com/gorilla/mux
RUN go get github.com/lib/pq
RUN go get github.com/chrismamo1/reflagvsflag
RUN apt-get update && apt-get install ruby
RUN gem install sass
RUN sass -v
CMD cd / && \
        rm -rf /go/src/github.com/chrismamo1/reflagvsflag/* && \
        rm -rf /go/src/github.com/chrismamo1/reflagvsflag/.git/* && \
        rmdir /go/src/github.com/chrismamo1/reflagvsflag/.git && \
        rmdir /go/src/github.com/chrismamo1/reflagvsflag && \
        go get github.com/chrismamo1/reflagvsflag && \
        cd /go/src/github.com/chrismamo1/reflagvsflag && \
        ls && \
        pwd && \
        make styles && \
        go run ./main.go
