#!/bin/bash

source="${1}"

for f in "${source}"/*.yaml; do
  target="${f/transforms/functions}"
  ./appcat convert --file "$f" > "${target}"
done
