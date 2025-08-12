#!/usr/bin/bash

# THis is my first GIT repo upload from scratch from sb01.montana.local

function DO_STUFF()
{
    X=${1}
    dd if=/dev/zero of=file.${i} bs=1M count=${X}
    return
}


## MAIN

TMPFILE1="/tmp/tempfile.$(date +%s.%N).1.tmp"

for i in $(seq -w 1 1 100) ; do 
    DO_STUFF ${i}
done
