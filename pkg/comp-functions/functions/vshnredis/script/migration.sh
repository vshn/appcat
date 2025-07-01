#!/bin/bash

set -x

echo "Copy data"

rsync -avhWHAX --no-compress --progress /src/ /dst/
