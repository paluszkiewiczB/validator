# Validator

## Install

```bash
go install github.com/paluszkiewiczB/validator
```

## Run

```bash
go run github.com/paluszkiewiczB/validator -in source.go -out destination.go -outpkg=mypackage
```

## Local

### Setup (once)

```shell
go install github.com/go-task/task/v3/cmd/task@v3.37.1
task setup
```

### Then do what you need

```bash
task
```
