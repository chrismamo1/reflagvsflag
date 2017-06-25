FROM chrismamo1/reflagvsflag:reflagvsflag-base
EXPOSE 80
RUN go get github.com/chrismamo1/reflagvsflag
WORKDIR /go/src/github.com/chrismamo1
ADD . /reflagvsflag
WORKDIR /go/src/github.com/chrismamo1/reflagvsflag
RUN ls && pwd
RUN make styles

WORKDIR /go/src/github.com/chrismamo1/reflagvsflag/views/react
RUN node --version
RUN npm --version
RUN npm install --only=dev
RUN npm run bsb-clean
RUN npm run bsb-world
RUN npm run bsb
RUN npm run dist

WORKDIR /go/src/github.com/chrismamo1/reflagvsflag
RUN make static/js/reflagvsflag.js
CMD ls && pwd && \
        go run ./main.go
