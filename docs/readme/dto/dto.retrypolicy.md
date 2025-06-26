## dto.RetryPolicy
RetryPolicy defines the configuration for retry attempts in case of failures.

| Field          | Description                                                                                                                                                  |
|----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `base-timeout` | BaseTimeout is the initial delay between retry attempts, in milliseconds.                                                                                    |
| `max-retries`  | MaxRetries is the maximum number of retry attempts that will be made.<br>If set to 0, no retries will be performed.                                          |
| `multiplier`   | Multiplier is used to increase the delay between subsequent retry attempts.<br>The actual delay is calculated as: BaseTimeout * (Multiplier ^ attemptNumber) |
