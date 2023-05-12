test-server:
	docker run -it --rm \
        -v ${PWD}/config:/config \
        -v ${PWD}/reports:/reports \
        crossbario/autobahn-testsuite \
        wstest -m fuzzingclient -s /config/fuzzingclient.json

test-client:
	docker run -it --rm \
        -v ${PWD}/config:/config \
        -v ${PWD}/reports:/reports \
        -p 9001:9001 \
        -p 9002:8080 \
        crossbario/autobahn-testsuite \
        wstest -m fuzzingserver -s /config/fuzzingserver.json
