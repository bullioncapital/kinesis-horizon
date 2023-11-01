ARG KINESIS_CORE_IMAGE=abxit/kinesis-core:v18.5.0-kinesis.1
ARG GO_VERSION=1.19
ARG VERSION=develop

FROM ${KINESIS_CORE_IMAGE} as core

FROM golang:${GO_VERSION} as builder
ENV DEBIAN_FRONTEND=noninteractive
ARG VERSION=develop
RUN apt-get update && apt-get upgrade -y && apt-get install -y --no-install-suggests --no-install-recommends

# This Dockerfile adds a non-root 'vscode' user with sudo access. However, for Linux,
# this user's GID/UID must match your local user UID/GID to avoid permission issues
# with bind mounts. Update USER_UID / USER_GID if yours is not 1000. See
# https://aka.ms/vscode-remote/containers/non-root-user for details.
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

# setup apt / certificates
RUN apt-get update \
    && apt-get -y install --no-install-recommends apt-utils dialog ca-certificates 2>&1

# ca-certificates are required to make tls connections
RUN apt-get install -y --no-install-recommends curl wget gnupg jq libpq5 git\
  && apt-get clean

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

# install PGSQL for integration testing
RUN apt-get update && apt-get install -y --no-install-suggests --no-install-recommends postgresql && apt-get clean
COPY ../pg_hba.conf /etc/postgresql/15/main/pg_hba.conf

# Set up locale
RUN sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen \
    && locale-gen

ENV LANG en_US.UTF-8
ENV LANGUAGE en_US:en
ENV LC_ALL en_US.UTF-8

COPY --from=core /usr/bin/stellar-core /usr/bin/stellar-core

