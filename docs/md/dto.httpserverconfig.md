## dto.HTTPServerConfig

| Field          | Description                                                                                | Default Value |
|----------------|--------------------------------------------------------------------------------------------|---------------|
| `address`      | The address to listen on.                                                                  | `0.0.0.0`     |
| `context-path` | ContextPath customizes path for the API endpoints.                                         | `/`           |
| `port`         | The port to listen on.                                                                     | `8080`        |
| `rate`         | HTTP rate limiter configuration.<br>See: [dto.RateLimiterConfig](dto.ratelimiterconfig.md) |               |
| `timeout`      | Timeout for http server operations in milliseconds.                                        | `5000`        |
