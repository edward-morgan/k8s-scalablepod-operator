#!/bin/bash

# Usage: bash add_scalablepod.sh sptest 5 | kubectl apply -f -

NAME=$1
TTL=$2

sp='apiVersion: "scalable.scalablepod.tutorial.io/v1"
kind: ScalablePod
metadata:
  name: '"$NAME"'
spec:
  podImageName: "busybox"
  podImageTag: "latest"
  maxActiveTimeSec: '"$TTL"'
'
echo "$sp"