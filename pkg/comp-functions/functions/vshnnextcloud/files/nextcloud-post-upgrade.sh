#!/usr/bin/env bash

/var/www/html/occ db:add-missing-indices
/var/www/html/occ app:update --all
