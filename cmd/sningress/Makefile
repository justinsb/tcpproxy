REGISTRY=justinsb/

push:
	cd ../../; docker buildx build --tag ${REGISTRY}sningress-server:latest --push -f cmd/sningress/Dockerfile .

bounce:
	kubectl delete pods -n sningress-system --all

logs:
	stern -n sningress-system ".*"
