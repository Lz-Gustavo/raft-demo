#!/bin/bash

DISKSTORAGE_STATE_FILE=/tmp/store1gb.txt
BEELOG_STATE=/tmp/beelog-*.log
APP_LOG=/tmp/log-file-*.log

echo "The following files will be permanently REMOVED:"
echo "$DISKSTORAGE_STATE_FILE"
echo "$BEELOG_STATE"
echo "$APP_LOG"
echo ""
echo "deleting in 3s ..."
sleep 3s

rm $DISKSTORAGE_STATE_FILE
rm $BEELOG_STATE
rm $APP_LOG
echo "finished!"
