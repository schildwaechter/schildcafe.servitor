# SchildCafé: Servitør

![SchildCafé](logo.png)

This is a [Go](https://go.dev/) implementation of the Servitør app used in the SchildCafé,
built with [Gin Gonic](https://gin-gonic.com/) and [GORM](https://gorm.io/).

## Installation

```bash
go mod tidy
```
or
```bash
make prep
```

## Options

To specify a port other then the default, set the environment variable
```bash
export SERVITOR_PORT="1333"
```

To log log in Gelf format, use the following environment setting
```bash
export GELF_LOGGING="true"
```

In order to turn off debug mode, use
```bash
export GIN_MODE="release"
```

## Execution

```bash
make run
```