FROM alpine:3.5
RUN apk add --update build-base m4 bash
RUN apk \
        add \
          aws-cli \
          --allow-untrusted \
          --update \
          --update-cache \
          --repository http://dl-3.alpinelinux.org/alpine/edge/testing/ && \
        apk add --update make opam nodejs patch && \
        opam init && \
        eval `opam config env` && \
        opam install camlp4 && \
        eval `opam config env` && \
        opam switch 4.03.0 && \
        eval `opam config env` && \
        opam install camlp4
RUN eval `opam config env` && \
        opam update && \
        opam install reason

RUN apk add curl wget ruby ruby-bundler ruby-dev ruby-rdoc ruby-irb && \
        gem install sass && \
        mkdir -p /home/views/react

COPY ./react/package.json /home/views/react/package.json
WORKDIR /home/views/react
RUN npm install --save-dev bs-platform && \
        npm install --only=dev

RUN apk add groff less python py-pip && \
        rm -rf /var/cache/apk/* && \
        pip install awscli
