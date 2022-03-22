# Config updater

Config updater is a service for dynamic config generation
via http URL and can be used by vmagent. 

How it works:
1. By default, service listens for http requests at `-http.listenAddr` (by default `:8436`);
2. It generates scrape configuration of `-config.targetsCount` targets
with `-config.targetAddr` address;
3. Generated config is available via HTTP on `/api/v1/config` path;
4. Every `-config.updateInterval` it changes a label value
for `-config.targetsToUpdatePercentage` of targets.

See full list of configuration flags by passing `-help` flag to the binary.