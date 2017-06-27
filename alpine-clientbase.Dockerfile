FROM alpine:3.5
ENV OPAMYES=true
COPY views/react /home/reflagvsflag
WORKDIR /home/reflagvsflag
RUN apk add m4 opam nodejs && \
        opam init && \
        eval `opam config env` && \
        opam switch 4.03.0 && \
        eval `opam config env` && \
        opam update && \
        opam upgrade && \
        opam install reason && \
        npm install --only=dev && \
        npm run bsb-clean && \
        npm run bsb-world && \
        npm run bsb && \
        npm run dist
