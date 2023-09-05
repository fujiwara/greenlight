# greenlight

greelight is a graceful health check agent.

## Usage

greenlight

## Configuration

```yaml
startup:
  checks:
    - name: "memcached alive"
      tcp:
        host: "localhost"
        port: 11211
        send: "stats\n"
        expect_pattern: "STAT uptime"
        quit: "QUIT\n"
    - name: "app server is up"
      command:
        run: 'curl -s -o /dev/null -w "%{http_code}" http://localhost:3000'
    - &web
      name: "web server is ok"
      http:
        url: "http://localhost:80/health"
        expect_code: 200-399
readiness:
  checks:
    - name: "pass file exists"
      command:
        run: "test -f pass"
    - *web
responder:
  addr: ":8081"
```

## LICENSE

MIT

## Author

fujiwara
