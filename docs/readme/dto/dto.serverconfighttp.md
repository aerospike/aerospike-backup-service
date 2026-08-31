## dto.ServerConfigHTTP
ServerConfigHTTP represents the service's HTTP server configuration.

| Field           | Description                                                                                                                                       | Default Value |
|-----------------|---------------------------------------------------------------------------------------------------------------------------------------------------|---------------|
| `address`       | The address to listen on.                                                                                                                         | `0.0.0.0`     |
| `context-path`  | ContextPath customizes path for the API endpoints.                                                                                                | `/`           |
| `disabled`      | Disabled controls whether the listener is disabled.                                                                                               | `false`       |
| `idle-timeout`  | IdleTimeout is the maximum amount of time in milliseconds to wait for the next request<br>when keep-alives are enabled (http.Server.IdleTimeout). | `120000`      |
| `port`          | The port to listen on.                                                                                                                            | `8080`        |
| `rate`          | HTTP rate limiter configuration.<br>See: [dto.RateLimiterConfig](dto.ratelimiterconfig.md)                                                        |               |
| `read-timeout`  | ReadTimeout is the maximum duration in milliseconds for reading the entire request,<br>including the body (http.Server.ReadTimeout).              | `30000`       |
| `timeout`       | Timeout for reading HTTP request headers in milliseconds (http.Server.ReadHeaderTimeout).                                                         | `5000`        |
| `write-timeout` | WriteTimeout is the maximum duration in milliseconds before timing out writes of the response<br>(http.Server.WriteTimeout).                      | `60000`       |
