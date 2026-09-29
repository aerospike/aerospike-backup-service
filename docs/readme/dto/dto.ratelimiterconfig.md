## dto.RateLimiterConfig
RateLimiterConfig is the HTTP server rate limiter configuration.

| Field        | Description                                                                                                                                                                                               | Default Value |
|--------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------|
| `size`       | Rate limiter token bucket size (bursts threshold).                                                                                                                                                        | `1024`        |
| `tps`        | Rate limiter tokens per second threshold.<br>The budget is per client: an IPv4 address gets its own, an IPv6 address shares one<br>with the rest of its /64.                                              | `1024`        |
| `white-list` | The list of ips to exempt from rate limiting (optional).<br>Default: empty list, so rate limiting applies to all clients.<br>Use "0.0.0.0/0" to exempt all clients and effectively disable rate limiting. |               |
