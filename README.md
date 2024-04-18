command faker

[![Go Report Card](https://goreportcard.com/badge/github.com/shu-go/faker)](https://goreportcard.com/report/github.com/shu-go/faker)
![MIT License](https://img.shields.io/badge/License-MIT-blue)

# Usage

## add (replace) a command

```
f --add gitinit git init
f --add goinit go mod init
```

## list commands

```
f
```

## run a command

```
f gitinit
```

## remove a command

```
f --remove gitinit
```

## add sub command

```
f --add m.c calc
f m c
```

## make another faker

```
copy f another.exe
another --add gitinit echo hoge hoge
```

## piping process

```
f --add clip cmd /c echo "|" clip

f clip abc
```

Args like "abc" above goes to the first command (echo).

# config dir:

1. exe path
   - f.yaml
   - Place the yaml in the same location as the executable.
2. config directory
   - {CONFIG_DIR}/faker/f.yaml
   - Windows: %appdata%\faker\f.yaml
   - (see https://cs.opensource.google/go/go/+/go1.17.3:src/os/file.go;l=457)

If none of 1,2 files exist, --add writes to 1.


<!-- vim: set et ft=markdown sts=4 sw=4 ts=4 tw=0 : -->
