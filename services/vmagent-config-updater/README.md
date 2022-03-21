

```vmagent-config-updater``` is a service which dynamically generates config for vmagent
and is available via HTTP.
How it works:
1. By default, service listens for http requests at ``;
2. Every time interval which you can define in [values.yaml](values.yaml) it updates label for some % of targets;
3. vmagent asks new config via `api/v1/config`, for example, every 10 seconds and updates it;
   How vmagent updates config you can check in the documentation
   [vmakert config update](https://docs.victoriametrics.com/vmagent.html#configuration-update)