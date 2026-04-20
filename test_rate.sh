#!/bin/bash


cd "/Users/priyankagajera/Desktop/job Queue"



curl --no-buffer -X POST http://localhost:8080/upload -F "file=@15MB_test.csv" &
PID1=$!

curl --no-buffer -X POST http://localhost:8080/upload -F "file=@15MB_test.csv" &
PID2=$!

curl --no-buffer -X POST http://localhost:8080/upload -F "file=@15MB_test.csv" &
PID3=$!

curl --no-buffer -X POST http://localhost:8080/upload -F "file=@15MB_test.csv" &
PID4=$!

wait $PID1 $PID2 $PID3 $PID4


