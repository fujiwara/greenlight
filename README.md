# greenlight

greelight is a graceful health check agent.

## Usage

greenlight

## Configuration

```yaml
startup:
  interval: 5s
  checks:
    - command: "test -f pass"
      timeout: 1s
    - command: 'curl -s -o /dev/null -w "%{http_code}" http://localhost:3000'
      timeout: 5s
# TODO
```

## LICENSE

MIT

## Author

fujiwara
