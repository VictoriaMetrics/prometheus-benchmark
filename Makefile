include services/vmagent-config-updater/Makefile

# k8s namespace for installing the chart
# It can be overriden via NAMESPACE environment variable
NAMESPACE ?= vm-benchmark

# the deployment prefix
CHART_NAME := rw-benchmark

# print resulting manifests to console without applying them
debug:
	helm install --dry-run --debug $(CHART_NAME) -n $(NAMESPACE) chart/

# install the chart to configured namespace
install:
	helm upgrade -i $(CHART_NAME) -n $(NAMESPACE) --create-namespace chart/

# delete the chart from configured namespace
delete:
	helm uninstall $(CHART_NAME) -n $(NAMESPACE)

monitor:
	kubectl port-forward `kubectl -n $(NAMESPACE) get pods -n $(NAMESPACE) -l 'job=vmsingle,chart-name=$(CHART_NAME)-prometheus-benchmark' -o name` 8428

re-install: delete install
