#!/bin/bash
set -e

if ! command -v go &> /dev/null
then
    echo "Error: Go not found in PATH"
    exit 1
fi

if ! command -v protoc &> /dev/null
then
    echo "Error: 'protoc' command not found"
    exit 1
fi

if ! command -v protoc-gen-go &> /dev/null || ! command -v protoc-gen-go-grpc &> /dev/null
then
    echo "Error: Go gRPC plugins were not found in PATH"
    exit 1
fi

FLUX_GO_CLIENT_DIR="./flux"
FLUX_PROTO_DIR="${FLUX_GO_CLIENT_DIR}/proto"
FLUX_GEN_OUTPUT="${FLUX_GO_CLIENT_DIR}/gen"
FLUX_PROTO_FILE="${FLUX_PROTO_DIR}/service_registry.proto"

mkdir -p "${FLUX_GEN_OUTPUT}"

cd "${FLUX_PROTO_DIR}"

protoc --go_out="${FLUX_GEN_OUTPUT}" --go_opt=paths=source_relative \
       --go-grpc_out="${FLUX_GEN_OUTPUT}" --go_opt=paths=source_relative \
       "${FLUX_PROTO_FILE}"

go mod tidy

echo "Protobuf stubs have been generated for flux"