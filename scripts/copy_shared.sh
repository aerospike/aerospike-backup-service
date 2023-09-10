#!/bin/bash
set -e

cd ..
[ -d lib ] || mkdir lib
cp ./modules/aerospike-tools-backup/bin/asbackup.dylib ./lib/libasbackup.dylib
cp ./modules/aerospike-tools-backup/bin/asrestore.dylib ./lib/libasrestore.dylib