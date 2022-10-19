include services/vmagent-config-updater/Makefile

# k8s namespace for installing the chart
# It can be overriden via NAMESPACE environment variable
NAMESPACE ?= vm-benchmark

# the deployment prefix
CHART_NAME := bench-100

# print resulting manifests to console without applying them
debug:
	helm install --dry-run --debug $(CHART_NAME) -n $(NAMESPACE) chart/ -f bench-overrides.yaml

# install the chart to configured namespace
install:
	helm upgrade -i $(CHART_NAME) -n $(NAMESPACE) --create-namespace chart/ -f bench-overrides.yaml

# delete the chart from configured namespace
delete:
	helm uninstall $(CHART_NAME) -n $(NAMESPACE)

monitor:
	kubectl -n $(NAMESPACE) port-forward deployment/$(CHART_NAME)-prometheus-benchmark-vmsingle 8428

re-install: delete install
