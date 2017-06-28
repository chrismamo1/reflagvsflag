FROM alpine:3.5
ENV OPAMYES=true
RUN mkdir -p /home/views/react
COPY ./react/package.json /home/views/react/package.json
RUN apk add --update build-base
RUN apk \
        add \
          aws-cli \
          --allow-untrusted \
          --update-cache \
          --repository http://dl-3.alpinelinux.org/alpine/edge/testing/ && \
        rm -rf /var/cache/apk/*
RUN apk add gcc m4 make opam nodejs patch && \
        rm -rf /var/cache/apk/*
RUN opam init && \
        eval `opam config env` && \
        opam install camlp4
RUN eval `opam config env` && \
        opam switch 4.03.0
RUN eval `opam config env` && \
        opam install camlp4
RUN eval `opam config env` && \
        opam update && \
        opam upgrade
RUN eval `opam config env` && \
        opam install reason

RUN apk add curl wget ruby ruby-bundler && \
        rm -rf /var/cache/apk/*
RUN gem install sass && \
        echo Sass version: `sass -v`
WORKDIR /home/views/react
RUN npm install --only=dev
