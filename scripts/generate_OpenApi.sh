#!/bin/bash

# Check if the "swag" command exists
if ! command -v swag &> /dev/null
then
    echo "swag is not installed. Installing..."

    # Install swag
    go get -u github.com/swaggo/swag/cmd/swag

    # Check if the installation was successful
    if [ $? -eq 0 ]
    then
        echo "swag installed successfully."
    else
        echo "Error: Failed to install swag. Please install it manually."
        exit 1
    fi
fi

swag init -d ../internal/server,../pkg/model -g server.go -o ../docs

