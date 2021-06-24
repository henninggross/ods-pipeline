#!/usr/bin/env bash
set -ue

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ODS_PIPELINE_DIR=${SCRIPT_DIR%/*}

INSECURE=""
HOST_PORT="9000"
SONAR_USERNAME="admin"
SONAR_PASSWORD="admin"

while [[ "$#" -gt 0 ]]; do
    case $1 in

    -v|--verbose) set -x;;

    -i|--insecure) INSECURE="--insecure";;

    *) echo "Unknown parameter passed: $1"; exit 1;;
esac; shift; done


SONARQUBE_URL="http://localhost:${HOST_PORT}"
echo "Waiting up to 4 minutes for SonarQube to start ..."
n=0
health="RED"
set +e
until [ $n -ge 24 ]; do
    health=$(curl -s ${INSECURE} --user "${SONAR_USERNAME}:${SONAR_PASSWORD}" \
        "${SONARQUBE_URL}/api/system/health" | jq -r .health)
    if [ "${health}" == "GREEN" ]; then
        echo " success"
        break
    else
        echo -n "."
        sleep 10s
        n=$((n+1))
    fi
done
set -e
if [ "${health}" != "GREEN" ]; then
    echo "SonarQube did not start, got health=${health}."
    exit 1
fi
