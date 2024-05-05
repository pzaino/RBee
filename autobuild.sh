#!/bin/bash

# This script is used to build the project automatically.
build_objs="$1"
rval=0

# Utility functions
function checkError() {
    if [ "$?" != "0" ]; then
        echo "Error: $1"
        exit 1
    fi
}

function moveFile() {
    # Copy a file from one directory to another in a case-insensitive way
    # Usage: copyFile <source> <destination>
    local src_dir
    src_dir="$(dirname "$1")"
    local src_file
    src_file="$(basename "$1")"
    local dest
    dest="$2"

    # Find the file in a case-insensitive way and copy it
    find "$src_dir" -maxdepth 1 -iname "$src_file" -exec mv {} "$dest" \;
}

# Check if the go command is installed
which go
checkError "Go is not installed!"

# Clean up the bin directory
rm -rf ./bin
mkdir ./bin

# Build the project
if  [ "${build_objs}" == "all" ] ||
    [ "${build_objs}" == "" ] ||
    [ "${build_objs}" == "rb" ];
then
    cmd_name="rbee"
    CGO_ENABLED=1 go build ./cmd/${cmd_name}
    rval=$?
    if [ "${rval}" == "0" ]; then
        echo "${cmd_name} command line tool built successfully!"
        moveFile ${cmd_name} ./bin
    else
        echo "${cmd_name} command line tool build failed!"
        exit $rval
    fi
fi

exit "$rval"

# Path: autobuild.sh
