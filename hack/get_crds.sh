#!/bin/bash

url=${1}
name=${2}
src=${3}
dst=${4}

sed() {
    if hash gsed 2>/dev/null; then
        gsed "$@"
    else
        /usr/bin/sed "$@"
    fi
}

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

package="$(basename "${dst}")"
cp hack/generate.go.tmpl "${dst}/generate.go"

sed -i "s/REPLACEME/${package}/g" "${dst}/generate.go"
