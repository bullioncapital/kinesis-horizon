#!/bin/bash
# generate xdr source code from xdr files
# ------------------
# gem install bundler:2.2.15
# bundle install
# rake xdr:generate
# exit
# ------------------
# chown $USER xdr/xdr_generated.go

docker run --rm -it \
    -v "$PWD":/working \
    -w /working \
    --entrypoint bash \
    ruby:2.7.4-buster