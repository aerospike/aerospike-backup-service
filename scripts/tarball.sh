#!/bin/bash

WORKSPACE="$(git rev-parse --show-toplevel)"
BINARY_NAME="$(basename $WORKSPACE)"
VERSION="$(cat $WORKSPACE/VERSION)"

cd "$WORKSPACE" && git archive \
--format=tar \
--prefix="$BINARY_NAME-$VERSION/" \
--output="/tmp/$BINARY_NAME-$VERSION.tar" create-rpm-package

cd - || exit

git submodule foreach --recursive | while read -r submodule_path; do
    path="$(echo $submodule_path | awk -F\' '{print $2}'| cut -d'/' -f2-)"
    name="$(basename $path)"
    cd "$WORKSPACE/$path" && git archive \
    --prefix="$BINARY_NAME-$VERSION/$path/" \
    --format=tar HEAD \
    --output="$WORKSPACE/$name.tar"

    tar --concatenate --file="/tmp/$BINARY_NAME-$VERSION.tar" "$WORKSPACE/$name.tar"
    rm -rf "$WORKSPACE/$name.tar"
done

gzip -9 -f "/tmp/$BINARY_NAME-$VERSION.tar"
