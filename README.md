# greenlight

greelight is a graceful health check agent.

## Usage

greenlight works as a health check agent for your application.

```console
$ greenlight -c config.yml
```

greenlight checks your application's health by `startup` and `readiness` checks.

### startup

`startup` checks are executed at greenlight starts.

All the checks are passed, and greenlight starts a responder http server that responds `200 OK` to `GET /` request.

### readiness

`readiness` checks are executed while the greenlight is running periodically.

If some of the checks are failed, the responder http server and responds `503 Service Unavailable` to `GET /` request.

If all the checks are passed, the responder http server responds `200 OK` to `GET /` request.

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
