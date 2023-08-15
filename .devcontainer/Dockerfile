ARG KINESIS_CORE_IMAGE=abxit/kinesis-core:v18.5.0-kinesis.1
ARG GO_VERSION=1.19
ARG VERSION=develop

# FROM golang:${GO_VERSION} as builder
# ENV DEBIAN_FRONTEND=noninteractive
# ARG VERSION=develop
# RUN apt-get update && apt-get upgrade -y && apt-get install -y --no-install-suggests --no-install-recommends

# # # install PGSQL for integration testing
# # RUN apt-get install -y --no-install-suggests --no-install-recommends postgresql && apt-get clean
# # COPY ../pg_hba.conf /etc/postgresql/12/main/pg_hba.conf

# # install modules
# WORKDIR /src
# COPY ../go.mod ../go.sum ./
# RUN go mod download

# COPY ../ .
# RUN go mod download
# # skip tests
# # RUN /etc/init.d/postgresql start 11 && \
# #     go test -race -cover ./...

 
# ENV CGO_ENABLED=0
# ENV HORIZON_VERSION="-X=github.com/stellar/go/support/app.version=$VERSION"
# ENV GOFLAGS="-ldflags=$HORIZON_VERSION"
# RUN go install ./...


FROM ${KINESIS_CORE_IMAGE} as core

FROM ubuntu:20.04
ENV DEBIAN_FRONTEND=noninteractive

EXPOSE 8000
EXPOSE 9473


# # This Dockerfile adds a non-root 'vscode' user with sudo access. However, for Linux,
# # this user's GID/UID must match your local user UID/GID to avoid permission issues
# # with bind mounts. Update USER_UID / USER_GID if yours is not 1000. See
# # https://aka.ms/vscode-remote/containers/non-root-user for details.
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

# setup apt / certificates
RUN apt-get update \
    && apt-get -y install --no-install-recommends apt-utils dialog ca-certificates 2>&1

# ca-certificates are required to make tls connections
RUN apt-get install -y --no-install-recommends curl wget gnupg jq libpq5 git\
  && apt-get clean

# Not working
# RUN apt-get install -y gccgo-5 &&\
#    update-alternatives --set go /usr/bin/go-5 &&\
#    apt-get clean
# ENV GOROOT_BOOTSTRAP=/usr

#----
# Not working
# RUN apt-get update && apt-get install -y gvm &&\
#    apt-get clean

# install PGSQL for integration testing
RUN apt-get install -y --no-install-suggests --no-install-recommends postgresql && apt-get clean
COPY ../pg_hba.conf /etc/postgresql/12/main/pg_hba.conf

# RUN curl -O https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz && \
#     tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz && \
#     rm go${GO_VERSION}.linux-amd64.tar.gz

RUN wget https://go.dev/dl/go1.19.linux-amd64.tar.gz && \
   rm -rf /usr/local/go && tar -C /usr/local -xzf go1.19.linux-amd64.tar.gz 

RUN rm go1.19.linux-amd64.tar.gz 

ENV PATH $PATH:/usr/local/go/bin    


# install base container packages and prep for VSCode
  # Verify process tools, lsb-release (common in install instructions for CLIs) installed
RUN apt-get -y install iproute2 procps lsb-release \
    #
    # Create a non-root user to use if preferred - see https://aka.ms/vscode-remote/containers/non-root-user.
    && groupadd --gid $USER_GID $USERNAME \
    && useradd -s /bin/bash --uid $USER_UID --gid $USER_GID -m $USERNAME \
    # [Optional] Add sudo support for the non-root user
    && apt-get install -y sudo \
    && echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME\
    && chmod 0440 /etc/sudoers.d/$USERNAME

RUN apt-get install -y --no-install-recommends vim && apt-get update

# Switch back to dialog for any ad-hoc use of apt-get
ENV DEBIAN_FRONTEND=

COPY --from=core /usr/bin/stellar-core /usr/bin/stellar-core
# 


RUN /etc/init.d/postgresql start 11 
# && \go version
#  RUN go mod download

WORKDIR /src
COPY ../go.mod ../go.sum ./
RUN go mod download 

COPY ../ .
RUN go mod download


ENV CGO_ENABLED=0
ENV HORIZON_VERSION="-X=github.com/stellar/go/support/app.version=$VERSION"
ENV GOFLAGS="-ldflags=$HORIZON_VERSION"
RUN go install ./...

# ENV GOROOT_BOOTSTRAP="/usr/local/go/"
# RUN cd /usr/local/go/src && ./make.bash

