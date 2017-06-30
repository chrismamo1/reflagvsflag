FROM chrismamo1/reflagvsflag:alpine-clientbase
COPY . /home/views
WORKDIR /home/views/react
RUN eval `opam config env` && \
        npm run bsb-clean && \
        npm run bsb-world && \
        npm run bsb && \
        npm run dist && \
        aws s3 cp dist/ s3://reflagvsflag-static-files/scripts/ --exclude "*" --include "dist/*.js"
WORKDIR /home/views/css
RUN pwd
RUN sass *.scss && \
        aws s3 cp . s3://reflagvsflag-static-files/styles/ --exclude "*" --include *.css
