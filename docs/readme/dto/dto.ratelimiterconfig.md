## dto.RateLimiterConfig
RateLimiterConfig is the HTTP server rate limiter configuration.

| Field        | Description                                                                      | Default Value |
|--------------|----------------------------------------------------------------------------------|---------------|
| `size`       | Rate limiter token bucket size (bursts threshold).                               | `1024`        |
| `tps`        | Rate limiter tokens per second threshold.                                        | `1024`        |
| `white-list` | The list of ips to whitelist in rate limiting (optional).<br>Default: allow all. |               |
