#!/bin/bash

if [[ $# -ne 3 ]]
then
	echo "usage: $0 numberClients reqForeachClient diffHashKeys"
	exit 1
fi

#go test -v -count 1 -clients=${1} -req=${2} -key=${3} >> result.txt
go test -count 1 -clients=${1} -req=${2} -key=${3}