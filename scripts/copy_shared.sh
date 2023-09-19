#!/bin/bash
set -e

ROOT_PATH=$(cd `dirname $0` && pwd)/..
[ -d $ROOT_PATH/lib ] || mkdir $ROOT_PATH/lib
cp -f $ROOT_PATH/modules/aerospike-tools-backup/bin/asbackup.dylib $ROOT_PATH/lib/libasbackup.dylib
cp -f $ROOT_PATH/modules/aerospike-tools-backup/bin/asrestore.dylib $ROOT_PATH/lib/libasrestore.dylib
