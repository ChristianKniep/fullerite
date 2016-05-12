#!/bin/bash

docker run -ti -v $(pwd):/data/ -w /data/ qnib/golang make
mv bin/fullerite bin/fullerite-$1-Linux
rm -f bin/gom bin/beatit

docker run -ti -v $(pwd):/data/ -w /data/ qnib/alpn-go-dev make
mv bin/fullerite bin/fullerite-$1-LinuxMusl
rm -f bin/gom bin/beatit

