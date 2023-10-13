#!/bin/bash

# Check if the "swag" command exists
if ! command -v swag &> /dev/null
then
    echo "swag is not installed. Installing..."

    # Install swag
    go install github.com/swaggo/swag/cmd/swag@latest

    # Check if the installation was successful
    if [ $? -eq 0 ]
    then
        echo "swag installed successfully."
    else
        echo "Error: Failed to install swag. Please install it manually."
        exit 1
    fi
fi

ROOT_PATH=$(cd `dirname $0` && pwd)/..
swag init -d $ROOT_PATH/internal/server,$ROOT_PATH/pkg/model \
    -g server.go \
    -o $ROOT_PATH/docs
