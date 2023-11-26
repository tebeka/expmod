# expmod - Explain Go Mod

[![Tests](https://github.com/tebeka/expmod/actions/workflows/test.yml/badge.svg)](https://github.com/tebeka/expmod/actions/workflows/test.yml)

Prints GitHub project description for every direct dependency on GitHub in go.mod.

## Usage

```
usage: expmod [options] [file or URL]
Options:
  -timeout duration
    	HTTP timeout (default 3s)
  -version
    	show version and exit

If GITHUB_TOKEN is found in the environment, it will be use to access GitHub API.
"Human" GitHub URLs (e.g. https://github.com/tebeka/expmod/blob/main/go.mod) will be redirected to raw content.
```


## Example

```
$ expmod go.mod 
github.com/sahilm/fuzzy v0.1.0:
	Go library that provides fuzzy string matching optimized for filenames and code symbols in the style of Sublime Text, VSCode, IntelliJ IDEA et al.
github.com/stretchr/testify v1.8.4:
	A toolkit with common assertions and mocks that plays nicely with the standard library
```

## Install

You can get the tool from the [GitHub release section](https://github.com/tebeka/expmod/releases), or:

```
$ go install github.com/tebeka/expmod@latest
```

Make sure `$(go env GOPATH)/bin` is in your `$PATH`.
