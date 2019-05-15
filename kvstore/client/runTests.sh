#!/bin/bash

if [[ $# -ne 4 ]]
then
	echo "usage with 4 arguments: $0 'mode(0: numMessages; 1: execTime)' 'numberClients' '(reqForeachClient||execTime)' 'diffHashKeys'"
	exit 1
fi

go clean -testcache
if [ $1 -eq "0" ]; then
	go test -run TestNumMessagesKvstore -count 1 -clients=${2} -req=${3} -key=${4}

elif [ $1 -eq "1" ]; then
	go test -run TestClientTimeKvstore -count 1 -clients=${2} -time=${3} -key=${4}
fi