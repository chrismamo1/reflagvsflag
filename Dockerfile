FROM chrismamo1/reflagvsflag:reflagvsflag-base
EXPOSE 80
RUN ls
RUN pwd
RUN go get github.com/chrismamo1/reflagvsflag
RUN ls
RUN pwd
CMD pwd && ls && \
        cd / && \
        rm -rf /go/src/github.com/chrismamo1/reflagvsflag/* && \
        rm -rf /go/src/github.com/chrismamo1/reflagvsflag/.git/* && \
        rmdir /go/src/github.com/chrismamo1/reflagvsflag/.git && \
        rmdir /go/src/github.com/chrismamo1/reflagvsflag && \
        go get github.com/chrismamo1/reflagvsflag && \
        cd /go/src/github.com/chrismamo1/reflagvsflag && \
        ls && \
        pwd && \
        make static/js/reflagvsflag.js && \
        make styles && \
        go run ./main.go
