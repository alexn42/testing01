#!/usr/bin/bash

# THis is my first GIT repo upload from scratch from sb01.montana.local

function STUFF(){

}

## MAIN

TMPFILE1="/tmp/tempfile.$(date +%s.%N).1.tmp"

for i in $(seq -w 1 1 100) ; do 

dd if=/dev/zero of=file.${i} bs=1M count=10


done
