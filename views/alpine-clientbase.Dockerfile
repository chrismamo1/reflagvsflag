FROM alpine:3.5
ENV OPAMYES=true
RUN mkdir -p /home/reflagvsflag
RUN apk add --update build-base
RUN apk \
        add \
        aws-cli \
        --allow-untrusted \
        --update-cache \
        --repository http://dl-3.alpinelinux.org/alpine/edge/testing/
RUN apk add gcc m4 make opam nodejs patch && \
        rm -rf /var/cache/apk/* && \
        opam init && \
        eval `opam config env` && \
        opam install camlp4 && \
        opam switch 4.03.0 && \
        eval `opam config env` && \
        opam install camlp4 && \
        opam update && \
        opam upgrade && \
        opam install reason && \
        npm install --only=dev
RUN apk add curl wget ruby ruby-bundler && \
        rm -rf /var/cache/apk/* && \
        gem install sass && \
        echo Sass version: `sass -v`
