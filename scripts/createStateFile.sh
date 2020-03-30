#!/bin/bash

STATE_FILE=/tmp/store1gb.txt

if test -f "$STATE_FILE"; then
    echo "$STATE_FILE already exists, deleting in 3s..."
	sleep 3s
	rm $STATE_FILE
	echo "$STATE_FILE succesfully removed."

fi

echo "Creating $STATE_FILE..."
dd if=/dev/zero of=$STATE_FILE count=1024 bs=1048576
echo "File succesfully created."
