# Sutro

A Command-line client for Strava.

**Disclaimer**: Extremely experimental, incomplete and unstable at this time

## Requirements

Sutro is a Go application and requires a Go development toolchain to be installed.

Sutro requires users to provision an application on the Strava website. Please see [this page](http://developers.strava.com/docs/getting-started/#account) for details. Once you have an application, you will need its id and secret to authenticate.

Finally, Sutro uses Go Swagger to generate the client code [installed](https://goswagger.io/install.html)

## Compiling

```sh
$ git submodule init
$ git submodule update
$ go generate ./...
$ go build -o sutro main.go
$ ./sutro -h
Usage:
  sutro [command]

Available Commands:
  authenticate Authentication support
  help         Help about any command

Flags:
  -h, --help   help for sutro

Use "sutro [command] --help" for more information about a command.
```

## Authenticating

Before you can execute any API calls in Sutro, you first need to provision an authentication token. You will need your application id and secret:

```sh
$ ./sutro authenticate \
  --client_id <client_id> \
  --client_secret <client_secret> \
  --authorization_url https://www.strava.com/oauth/authorize \
  --token_url https://www.strava.com/oauth/token \
  --scopes activity:read_all,activity:write,read_all,profile:read_all
```

The credentials, which include the application secret, will be stored in ~/.sutro. They will auto-refresh as needed, so you shouldn't need to run the authentication flow more than once. Once you've authenticated, you have access to the full API:

```sh
$ ./sutro
Usage:
  sutro [command]

Available Commands:
  activities      Client for activities
  athletes        Client for athletes
  authenticate    Authentication support
  clubs           Client for clubs
  gears           Client for gears
  help            Help about any command
  routes          Client for routes
  running_races   Client for running_races
  segment_efforts Client for segment_efforts
  segments        Client for segments
  streams         Client for streams
  uploads         Client for uploads

Flags:
  -h, --help   help for sutro

Use "sutro [command] --help" for more information about a command.
```
