#!/usr/bin/env bash

CODIS_ADMIN="${BASH_SOURCE-$0}"
CODIS_ADMIN="$(dirname "${CODIS_ADMIN}")"
CODIS_ADMIN_DIR="$(cd "${CODIS_ADMIN}"; pwd)"

CODIS_BIN_DIR=$CODIS_ADMIN_DIR/../bin
CODIS_LOG_DIR=$CODIS_ADMIN_DIR/../log
CODIS_CONF_DIR=$CODIS_ADMIN_DIR/../config

CODIS_BACKUP_BIN=$CODIS_BIN_DIR/codis-backup
CODIS_BACKUP_PID_FILE=$CODIS_BIN_DIR/codis-backup.pid

CODIS_BACKUP_LOG_FILE=$CODIS_LOG_DIR/codis-backup.log
CODIS_BACKUP_DAEMON_FILE=$CODIS_LOG_DIR/codis-backup.out

CODIS_BACKUP_CONF_FILE=$CODIS_CONF_DIR/backup.toml

echo $CODIS_BACKUP_CONF_FILE

if [ ! -d $CODIS_LOG_DIR ]; then
    mkdir -p $CODIS_LOG_DIR
fi


case $1 in
start)
    echo  "starting codis-backup ... "
    if [ -f "$CODIS_BACKUP_PID_FILE" ]; then
      if kill -0 `cat "$CODIS_BACKUP_PID_FILE"` > /dev/null 2>&1; then
         echo $command already running as process `cat "$CODIS_BACKUP_PID_FILE"`. 
         exit 0
      fi
    fi
    nohup "$CODIS_BACKUP_BIN" "--config=${CODIS_BACKUP_CONF_FILE}" "--log=$CODIS_BACKUP_LOG_FILE" "--log-level=INFO" "--pidfile=$CODIS_BACKUP_PID_FILE" > "$CODIS_BACKUP_DAEMON_FILE" 2>&1 < /dev/null & 
    ;;
start-foreground)
    $CODIS_BACKUP_BIN "--config=${CODIS_BACKUP_CONF_FILE}" "--log-level=DEBUG" "--pidfile=$CODIS_BACKUP_PID_FILE"
    ;;
stop)
    echo "stopping codis-backup ... "
    if [ ! -f "$CODIS_BACKUP_PID_FILE" ]
    then
      echo "no codis-backup to stop (could not find file $CODIS_BACKUP_PID_FILE)"
    else
      kill -2 $(cat "$CODIS_BACKUP_PID_FILE")
      echo STOPPED
    fi
    exit 0
    ;;
stop-forced)
    echo "stopping codis-backup ... "
    if [ ! -f "$CODIS_BACKUP_PID_FILE" ]
    then
      echo "no codis-backup to stop (could not find file $CODIS_BACKUP_PID_FILE)"
    else
      kill -9 $(cat "$CODIS_BACKUP_PID_FILE")
      rm "$CODIS_BACKUP_PID_FILE"
      echo STOPPED
    fi
    exit 0
    ;;
restart)
    shift
    "$0" stop
    sleep 1
    "$0" start
    ;;
*)
    echo "Usage: $0 {start|start-foreground|stop|stop-forced|restart}" >&2

esac
