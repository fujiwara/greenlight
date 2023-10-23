# greenlight

greelight is a graceful health check agent.

## Usage

```
Usage: greenlight --config="greenlight.yaml" [<child-cmds> ...]

Arguments:
  [<child-cmds> ...]

Flags:
  -h, --help                        Show context-sensitive help.
  -c, --config="greenlight.yaml"    config file path or URL(http,https,file,s3) ($GREENLIGHT_CONFIG)
  -d, --debug                       debug mode ($GREENLIGHT_DEBUG)
      --version                     show version
```

greenlight works as a health check agent for your application.

greenlight checks your application's health by `startup` and `readiness` checks.

## Install

### Homebrew

```console
$ brew install fujiwara/tap/greenlight
```

### Binary

[Releases](https://github.com/fujiwara/greenlight/releases)

### Docker images

[ghcr.io/fujiwara/greenlight](https://github.com/fujiwara/greenlight/pkgs/container/greenlight)

```console
docker pull ghcr.io/fujiwara/greenlight:v0.0.5
```

```Dockerfile
FROM ghcr.io/fujiwara/greenlight:v0.0.5 AS greenlight

FROM debian:bookworm-slim
COPY --from=greenlight /usr/local/bin/greenlight /usr/local/bin/greenlight
COPY greenlight.yaml /etc/greenlight.yaml
ENV GREENLIGHT_CONFIG=/etc/greenlight.yaml
CMD ["/usr/local/bin/greenlight", "--", "/path/to/your/app"]
```

## Spawn a child process

greenlight can spawn a child process (optional).

```conosole
$ greenlight --config=greenlight.yaml -- /path/to/your/app
```

greenlight spawns a child process at first.

When greenlight catch a signal (SIGTERM and SIGINT), greenlight sends SIGTERM to the child process, and waits for the child process to exit. If the child process does not exit in 30 seconds, greenlight sends SIGKILL to the child process.

STDOUT and STDERR of the child process are redirected to greenlight's STDOUT and STDERR.

## Health checks

greenlight checks your application's health by `startup` and `readiness` checks.

### startup

Startup checks are executed at greenlight starts.

All the checks are passed, and greenlight starts a responder http server that responds `200 OK` to `GET /` request.

Startup checks are executed in a defined order in the configuration file. If some check fails, greenlight retries the check until the check is passed.

### readiness

Readiness checks are executed periodically while the greenlight is running.

If some checks fail, the responder returns `503 Service Unavailable` to `GET /` request.

If all the checks are passed, the responder returns `200 OK` to `GET /` request.

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

### `startup` section

```yaml
startup:
  interval: 10s # default 5s
  grace_period: 10s # default 0
  checks:
    - name: "memcached alive"
      timeout: 30s # default 5s
      tcp:
        host: "localhost"
        port: 11211
        send: "version\n"
        expect_pattern: "VERSION 1."
```

#### `startup.interval`

The retry interval when all the checks are not passed.

#### `startup.grace_period`

The grace period before starting the checks.

#### `startup.checks`

See [Check](#check) section.

### `readiness` section

```yaml
  interval: 10s # default 5s
  grace_period: 10s # default 0
  checks:
    - name: "app server alive"
      timeout: 10s # default 5s
      http:
        url: "http://localhost:3000/health"
        method: "GET" # default "GET"
        headers:
          Host: "example.com"
        expect_code: 200-299 # default 200-399
        expect_pattern: "OK" # matches a regexp with response body
```

#### `readiness.interval`

The interval to execute the checks.

#### `readiness.grace_period`

The grace period before starting the checks.

#### `readiness.checks`

See [Check](#check) section.

### `responder` section

```yaml
responder:
  addr: ":8081" # default ":8080"
```

### Check

`check` section defines a health check.

#### command check

```yaml
name: "pid file exists"
command:
  run: "test -f /var/run/app.pid"
```
command check executes a command and checks the exit status.
exit status 0 is passed, otherwise failed.

The command is not executed in a shell, so you can't use shell features like `&&` or `|`.
If you want to use shell features, use `sh -c` like this:

```yaml
name: "pid file exists"
command:
  run: "sh -c 'test -f /var/run/app.pid && test -f /var/run/app.pid'"
```
#### tcp check

```yaml
name: "memcached alive"
tcp:
  host: "localhost"
  port: 11211
  send: "version\n"
  expect_pattern: "VERSION 1."
  quit: "quit\n"
```

tcp check connects to the host and port.

Optionally, sends the `send` string, and checks the response matches the `expect_pattern` regexp.

If `quit` is defined, sends the `quit` string and closes the connection.

You can use TLS by `tls` section.

```yaml
name: "LDAPS server alive"
tcp:
  host: "localhost"
  port: 636
  tls: true
  no_check_certificate: true
```

If `no_check_certificate` is true, the certificate is not checked.

#### http check

```yaml
name: "app server alive"
http:
  url: "http://localhost:3000/health"
  method: "GET" # default "GET"
  headers:
    Host: "example.com"
  expect_code: 200-299 # default 200-399
  expect_pattern: "OK" # matches a regexp with response body
```

```yaml
name: "app server alive"
http:
  url: "https://localhost:3000/post"
  method: "POST"
  headers:
    Host: "example.com"
    Content-Type: "application/json"
  body: '{"foo":"bar"}'
  expect_code: 200-299 # default 200-399
  expect_pattern: "OK"
  no_check_certificate: true
```

http check sends a http request to the url, and checks the response code.

Optionally, checks the response body matches the `expect_pattern` regexp.

`expect_code` can be a range like `200-299`, or a list like `200,201,202` or complex of them. For example, `200-299,301,302` is valid.

If `body` is defined, sends the body (for POST, PUT, etc).

If `no_check_certificate` is true, the certificate is not checked.

#### `responder.addr`

The address to listen by responder.

## LICENSE

MIT

## Author

fujiwara
