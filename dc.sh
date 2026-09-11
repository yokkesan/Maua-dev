#!/bin/bash

docker exec -it "auma_dev_$1" bash 2>/dev/null \
|| docker exec -it "auma_dev_$1" sh

# dc backend
# dc frontend
# dc db
# dc nginx