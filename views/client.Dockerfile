FROM chrismamo1/reflagvsflag:alpine-clientbase
COPY . /home/views
WORKDIR /home/views/react
RUN eval `opam config env` && \
        npm run bsb-clean && \
        npm run bsb-world && \
        npm run bsb && \
        npm run dist && \
        aws cp dist/ s3://reflagvsflag-static-files/scripts/ --exclude "*" --include "*.js"
WORKDIR /home/views/css
RUN pwd
RUN sass --update scss:css && \
        aws s3 cp . s3://reflagvsflag-static-files/styles/ --exclude "*" --include "*.css"
