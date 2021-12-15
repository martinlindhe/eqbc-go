docker_build:
	docker build -t martinlindhe/eqbc-go:latest .

docker_publish: docker_build
	docker image push martinlindhe/eqbc-go:latest
