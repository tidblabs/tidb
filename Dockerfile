# Copyright 2019 PingCAP, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Builder image
FROM golang:1.19.4-bullseye as builder

RUN apt install -y wget make git gcc && mkdir -p /go/src/github.com/pingcap/tidb

WORKDIR /go/src/github.com/pingcap/tidb

# Cache dependencies
COPY go.mod .
COPY go.sum .
COPY parser/go.mod parser/go.mod
COPY parser/go.sum parser/go.sum

ARG GITHUB_TOKEN
RUN if [ -n "$GITHUB_TOKEN" ]; then git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"; fi

ENV GOPRIVATE=github.com/tidbcloud
# Overwrite the minimum required tikv version to be compatible with cloud-storage-engine
ENV MIN_TIKV_VERSION=6.1.0
ENV REGISTER_METRICS_INIT=false
ENV TIDB_VERSION=v6.4.0-serverless
ENV TIDB_GIT_BRANCH=release-6.4-serverless

RUN GO111MODULE=on go mod download

# Build real binaries
COPY . .
RUN make

# Executable image
FROM debian:bullseye-slim
RUN apt update && apt install -y bash curl netcat dumb-init && rm /bin/sh && ln -s /bin/bash /bin/sh && apt-get clean

COPY --from=builder /go/src/github.com/pingcap/tidb/bin/tidb-server /tidb-server

WORKDIR /

EXPOSE 4000

ENTRYPOINT ["/usr/bin/dumb-init", "/tidb-server"]
