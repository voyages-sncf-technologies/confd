#!/bin/bash

set -o pipefail -o errexit -o nounset

HESPERIDES_ENDPOINT="http://docker.for.win.localhost:8080"
USER="tech"
PASSWORD="password"

export MOD_NAME=test-module
export MOD_VERSION=1.2.3
export APPLICATION=TEST-APP
export PLATFORM=TEST-PTF
export MODULE_PATH="#ROOT"
export PROPERTIES_PATH="$MODULE_PATH#$MOD_NAME#$MOD_VERSION#WORKINGCOPY"

cat <<EOF | curl --fail -u $USER:$PASSWORD "$HESPERIDES_ENDPOINT/rest/modules" -H 'Content-Type: application/json' -d @-
{
  "name": "$MOD_NAME",
  "version": "$MOD_VERSION",
  "working_copy": true,
  "technos": [],
  "version_id": 0
}
EOF
cat <<EOF | curl --fail -u $USER:$PASSWORD "$HESPERIDES_ENDPOINT/rest/modules/$MOD_NAME/$MOD_VERSION/workingcopy/templates" -H 'Content-Type: application/json' -d @-
{
  "name": "test-config-file",
  "location": "/tmp",
  "filename": "test-config-file",
  "content": "START: {{foo}}\nMIDDLE: \n{{fuzz|@password}}\nEND\n",
  "version_id": 0
}
EOF
cat <<EOF | curl --fail -u $USER:$PASSWORD "$HESPERIDES_ENDPOINT/rest/applications/$APPLICATION/platforms" -H 'Content-Type: application/json' -d @-
{
  "application_name": "$APPLICATION",
  "application_version": "1",
  "platform_name": "$PLATFORM",
  "production": false,
  "modules": [{
      "name": "$MOD_NAME",
      "version": "$MOD_VERSION",
      "working_copy": true,
      "path": "$MODULE_PATH",
      "properties_path": "$PROPERTIES_PATH",
      "instances": []
    }],
  "version_id": 0
}
EOF
cat <<EOF | curl -v --fail -u $USER:$PASSWORD "$HESPERIDES_ENDPOINT/rest/applications/$APPLICATION/platforms/$PLATFORM/properties?platform_vid=1&path=$(echo $PROPERTIES_PATH | sed 's/#/%23/g')" -H 'Content-Type: application/json' -d @-
{
  "iterable_properties": [],
  "key_value_properties": [
    {"name": "/database/host", "value": "127.0.0.1"},
    {"name": "/database/password", "value": "p@sSw0rd"},
    {"name": "/database/port", "value": "3306"},
    {"name": "/database/username", "value": "confd"},
    {"name": "HOSTNAME", "value": "127.0.0.1"}
  ]
}
EOF
cat <<EOF | curl -v --fail -u $USER:$PASSWORD "$HESPERIDES_ENDPOINT/rest/applications/$APPLICATION/platforms/$PLATFORM/properties?platform_vid=2&path=%23" -H 'Content-Type: application/json' -d @-
{
  "iterable_properties": [],
  "key_value_properties": [
    {"name": "/database/host", "value": "127.0.0.1"},
    {"name": "/database/password", "value": "p@sSw0rd"},
    {"name": "/database/port", "value": "3306"},
    {"name": "/database/username", "value": "confd"},
    {"name": "HOSTNAME", "value": "127.0.0.1"}
  ]
}
EOF

# Run confd
confd --onetime --log-level debug --confdir ./integration/confdir --backend hesperides \
    --node $HESPERIDES_ENDPOINT --username $USER --password $PASSWORD \
    --app $APPLICATION --platform $PLATFORM --watch

ls -l /tmp
cat /tmp/confd-basic-test.conf
cat /tmp/confd-hesperides-module-test.conf
