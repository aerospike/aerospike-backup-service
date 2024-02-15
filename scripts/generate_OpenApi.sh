#!/bin/bash

# Check if the swag installed
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
# Check if the openapi-generator installed
if ! command -v openapi-generator &> /dev/null
then
    if [[ ! -z $(which brew) ]]; then
        echo "openapi-generator is not installed. Installing now via Homebrew..."
        brew install openapi-generator
    else
        echo "Error: openapi-generator is not installed. Please install it manually."
        exit 1;
    fi
fi

ROOT_PATH=$(cd `dirname $0` && pwd)/..
swag init -d $ROOT_PATH/internal/server,$ROOT_PATH/pkg/model \
    -g server.go \
    -o $ROOT_PATH/docs

# swag codegen cannot handle int64 format for return values
sed -i '' 's/"type": "int64"/"type": "integer",\n                            "format": "int64"/g' $ROOT_PATH/docs/swagger.json
sed -i '' 's/"type": "int64"/"type": "integer",\n                            "format": "int64"/g' $ROOT_PATH/docs/docs.go
sed -i '' 's/type: int64/type: integer\n            format: int64/g' $ROOT_PATH/docs/swagger.yaml

openapi-generator generate -i $ROOT_PATH/docs/swagger.yaml -g openapi-yaml -o $ROOT_PATH/tmp
openapi-generator generate -i $ROOT_PATH/docs/swagger.json -g openapi -o $ROOT_PATH/tmp
mv $ROOT_PATH/tmp/openapi/openapi.yaml $ROOT_PATH/docs/openapi.yaml
mv $ROOT_PATH/tmp/openapi.json $ROOT_PATH/docs/openapi.json

rm -rf $ROOT_PATH/tmp $ROOT_PATH/docs/swagger.yaml $ROOT_PATH/docs/swagger.json
