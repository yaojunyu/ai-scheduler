#!/usr/bin/env bash

for (( c=1; c<=100; c++ ))
do
   cat jobs-test-for-qa.yaml | sed "s/test-job-i/test-job-$c/g" | kubectl delete -f -
done