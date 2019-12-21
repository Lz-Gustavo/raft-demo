#!/bin/bash

local=.
folders=("AppLogger" "DecoupLog" "NotLog")

numClients=(1 4 7 10 13 16 19)

dataSizeOptions=(0 1 2) #0: 128B, 1: 1KB, 2: 4KB

echo "creating experiment folders..."
for i in ${folders[*]}
do
	mkdir $local/${i}

	for j in ${dataSizeOptions[*]}
	do
		mkdir $local/${i}/${j}

		for k in ${numClients[*]}
		do
			mkdir $local/${i}/${j}/${k}
		done
	done
done

echo "finished!"