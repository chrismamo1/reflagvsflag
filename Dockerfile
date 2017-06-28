FROM chrismamo1/reflagvsflag:reflagvsflag-base
EXPOSE 80
RUN go get github.com/chrismamo1/reflagvsflag
WORKDIR /go/src/github.com/chrismamo1
ADD . /reflagvsflag

WORKDIR /go/src/github.com/chrismamo1/reflagvsflag/views/react
RUN eval `opam config env` && \
        node --version && \
        npm --version && \
        npm update && \
        npm upgrade --all
RUN eval `opam config env` && \
        npm update --all && \
        npm install --only=dev && \
        npm run bsb-clean && \
        npm run bsb-world && \
        npm run bsb && \
        npm run dist

WORKDIR /go/src/github.com/chrismamo1/reflagvsflag
RUN ls && pwd && \
        make static/js/reflagvsflag.js && \
        make styles
CMD ls && pwd && go run ./main.go
