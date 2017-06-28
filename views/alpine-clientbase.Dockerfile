FROM alpine:3.5
ENV OPAMYES=true
RUN mkdir -p /home/views/react
COPY ./react/package.json /home/views/react/package.json
RUN apk add --update build-base m4
RUN apk \
        add \
          aws-cli \
          --allow-untrusted \
          --update
          --update-cache \
          --repository http://dl-3.alpinelinux.org/alpine/edge/testing/
RUN apk add --update make opam nodejs patch
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

RUN apk add curl wget ruby ruby-bundler
RUN rm -rf /var/cache/apk/*
RUN gem install sass && \
        echo Sass version: `sass -v`
WORKDIR /home/views/react
RUN npm install --only=dev
