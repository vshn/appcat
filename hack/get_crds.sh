#!/bin/bash -xef

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
git clone --filter=tree:0 "$url" && \
cd "$name" && \
#git sparse-checkout set --no-cone apis && \
if [ -n "$5" ]; then git checkout "$5"; fi && \
#git checkout "$5" && \
find . -name "zz_generated*" -delete && \
rsync -ra --delete "$src" "../../$dst" && \
cd ../.. && rm -rf gen

package="$(basename "${dst}")"
cp hack/generate.go.tmpl "${dst}/generate.go"

sed -i "s/REPLACEME/${package}/g" "${dst}/generate.go"
