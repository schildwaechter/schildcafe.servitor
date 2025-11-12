# SchildCafé Servitør
# Copyright Carsten Thiel 2023
#
# SPDX-Identifier: Apache-2.0

prep:
	go mod tidy

lint:
	# linting and standard code vetting
	golint -set_exit_status . | tee golint-report.out
	go vet . 2> govet-report.out

test: prep lint
	# presentable output
	go test -v

build: prep
	swag init
	go build -v -o servitor .

run: build
	go run .

swagger:
	GO111MODULE=off swagger generate spec -o ./swagger.json --scan-models

