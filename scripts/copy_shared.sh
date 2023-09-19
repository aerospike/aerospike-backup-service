#!/bin/bash
set -e

ROOT_PATH=$(cd `dirname $0` && pwd)/..
[ -d lib ] || mkdir $ROOT_PATH/lib
cp $ROOT_PATH/modules/aerospike-tools-backup/bin/asbackup.dylib $ROOT_PATH/lib/libasbackup.dylib
cp $ROOT_PATH/modules/aerospike-tools-backup/bin/asrestore.dylib $ROOT_PATH/lib/libasrestore.dylib
