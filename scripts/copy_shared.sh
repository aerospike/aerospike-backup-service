#!/bin/bash
set -e

ROOT_PATH=$(cd `dirname $0` && pwd)/..
[ -d $ROOT_PATH/lib ] || mkdir $ROOT_PATH/lib

BIN_FOLDER=$ROOT_PATH/modules/aerospike-tools-backup/bin

[ -f $BIN_FOLDER/asbackup.so ] && cp -f $BIN_FOLDER/asbackup.so $ROOT_PATH/lib/libasbackup.so
[ -f $BIN_FOLDER/asrestore.so ] && cp -f $BIN_FOLDER/asrestore.so $ROOT_PATH/lib/libasrestore.so

[ -f $BIN_FOLDER/asbackup.dylib ] && cp -f $BIN_FOLDER/asbackup.dylib $ROOT_PATH/lib/libasbackup.dylib
[ -f $BIN_FOLDER/asrestore.dylib ] && cp -f $BIN_FOLDER/asrestore.dylib $ROOT_PATH/lib/libasrestore.dylib

echo "Done."
