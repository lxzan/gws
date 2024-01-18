test-server:
	docker run -it --rm \
	--network host \
	-v ${PWD}/config:/config \
	-v ${PWD}/reports:/reports \
	crossbario/autobahn-testsuite \
	wstest -m fuzzingclient -s /config/fuzzingclient.json

test-client:
	docker run -it --rm \
	--network host \
	-v ${PWD}/config:/config \
	-v ${PWD}/reports:/reports \
	crossbario/autobahn-testsuite \
	wstest -m fuzzingserver -s /config/fuzzingserver.json

clean:
	rm -f ./reports/servers/*
	rm -f ./reports/clients/*
