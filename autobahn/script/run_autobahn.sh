#!/bin/bash

echo "pwd:" $(pwd)
./autobahn/bin/autobahn_server &

# mkdir reports
rm -rf ${PWD}/autobahn/reports
mkdir -p ${PWD}/autobahn/reports/server
mkdir -p ${PWD}/autobahn/reports/client

docker run -it --rm \
    -v ${PWD}/autobahn/config:/config \
    -v ${PWD}/autobahn/reports:/reports \
    --network host \
    --name=autobahn \
    crossbario/autobahn-testsuite \
    wstest -m fuzzingclient -s /config/fuzzingclient.json


trap ctrl_c INT
ctrl_c () {
	echo "SIGINT received; cleaning up"
	docker kill --signal INT "autobahn" >/dev/null
	rm -rf ${PWD}/autobahn/bin
	rm -rf ${PWD}/autobahn/reports
	cleanup
	exit 130
} 

cleanup() {
	killall autobahn_server
}

./autobahn/bin/autobahn_reporter ${PWD}/autobahn/reports/server/index.json

cleanup

