#!/bin/bash

url=${1}
name=${2}
src=${3}
dst=${4}

rm -rf gen && \
mkdir gen && \
cd gen && \
git clone --depth 1 --filter=tree:0 "$url" && \
cd "$name" && \
git sparse-checkout set --no-cone apis && \
git checkout && \
find . -name "zz_generated*" -delete && \
rsync -ra --delete "$src" "../../$dst" && \
cd ../.. && rm -rf gen
