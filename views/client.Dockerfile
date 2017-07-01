FROM chrismamo1/reflagvsflag:alpine-clientbase
COPY . /home/views
WORKDIR /home/views/react
RUN echo AWS_ACCESS_KEY_ID $AWS_ACCESS_KEY_ID && \
        echo AWS_SECRET_ACCESS_KEY $AWS_SECRET_ACCESS_KEY && \
        eval `opam config env` && \
        npm run bsb-clean && \
        npm run bsb-world && \
        npm run bsb && \
        npm run dist && \
        aws s3 cp ./dist/reflagvsflag.js s3://reflagvsflag-static-files/scripts/reflagvsflag.js
WORKDIR /home/views/css
RUN pwd
RUN sass *.scss
RUN for file in ./*.css; do aws s3 cp $file s3://reflagvsflag-static-files/styles/; done
