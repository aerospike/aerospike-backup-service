#!/bin/bash
set -e

ROOT_PATH=$(cd `dirname $0` && pwd)/..
[ -d $ROOT_PATH/lib ] || mkdir $ROOT_PATH/lib
cp -f $ROOT_PATH/modules/aerospike-tools-backup/bin/asbackup.so $ROOT_PATH/lib/libasbackup.so
cp -f $ROOT_PATH/modules/aerospike-tools-backup/bin/asrestore.so $ROOT_PATH/lib/libasrestore.so
