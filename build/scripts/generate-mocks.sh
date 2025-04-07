#!/bin/sh
set -e

if ! command -v mockgen > /dev/null 2>&1; then
    go install go.uber.org/mock/mockgen@latest
fi

WORKSPACE="$(git rev-parse --show-toplevel)"
echo "Workspace root: $WORKSPACE"

# Extract module path
BASE_IMPORT_PATH=$(grep '^module' "$WORKSPACE/go.mod" | awk '{print $2}')
echo "Module path: $BASE_IMPORT_PATH"

generate_mocks() {
    _src_pkg_rel_path="$1"
    _interfaces="$2"

    # Derive output package name (last component of the source path)
    _out_pkg_name=$(basename "$_src_pkg_rel_path")

    # Derive output file path (always <package_dir>/mockgen.go)
    _full_out_file="$WORKSPACE/$_src_pkg_rel_path/mockgen.go"

    # Derive full source package import path
    _full_src_pkg_import="$BASE_IMPORT_PATH/$_src_pkg_rel_path"

    echo "--> Generating mocks for $_full_src_pkg_import"
    echo "    Output: $_full_out_file (package $_out_pkg_name)"

    mockgen -package "$_out_pkg_name" -destination "$_full_out_file" "$_full_src_pkg_import" "$_interfaces"

    echo "    Done."
}

generate_mocks \
    "pkg/service" \
    "RestoreManager,BackupReaderWriter,BackupReader"

generate_mocks \
    "pkg/service/restoreexecutor" \
    "Restore,RestoreHandler"

generate_mocks \
    "pkg/service/backupexecutor" \
    "Backup,BackupHandler"

generate_mocks \
    "pkg/service/aerospike" \
    "ClientManager,NamespaceValidator"

echo "All mocks generated successfully."