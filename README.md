# SchildCafé: Servitør

![SchildCafé](logo.png)

This is a [Go](https://go.dev/) implementation of the Servitør app used in the SchildCafé,
built with [Gin Gonic](https://gin-gonic.com/) and [GORM](https://gorm.io/).

## Installation

Consider using [Air](https://github.com/air-verse/air).

```shell
go mod tidy
```

or

```shell
make prep
```

## Compile

```shell
swag init
go run .
```

```shell
make run
```

## Options

To specify a port other then the default, set the environment variable

```shell
export SERVITOR_PORT="1333"
```

To log log in Gelf format, use the following environment setting

```shell
export GELF_LOGGING="true"
```

In order to turn off debug mode, use

```shell
export GIN_MODE="release"
```

To send traces to an OTEL endpoint, specify its address

```shell
export OTEL_TRACES_ENDPOINT="localhost:4318"
```

## Execution

Make sure to provide the MySQL credentials as environment variables.

```bash
make run
```
