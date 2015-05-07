#!/bin/bash
cores=$1
shift
((rend=cores-1))
echo numactl -C "0-$rend" env GOMAXPROCS=$cores "$@"
numactl -C "0-$rend" env GOMAXPROCS=$cores "$@"
