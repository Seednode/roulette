# Russian roulette for your media

## About

Sometimes, you just need a way to randomly display files from one or more directories in your browser.

This tool is here to help.

Simply point the tool at one or more directories, and then open the specified port (default 8080) in your browser to show random files.

A random file will be selected if you open the root URI directly (e.g. http://localhost:8080/), or if you click on any displayed image.

Browser history is preserved, so you can always go back to any previously displayed image.

Builds available [here](https://cdn.seedno.de/builds/roulette).

## Usage output
```
Usage:
  roulette <path1> [path2] ... [pathN] [flags]
  roulette [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  version     Print version

Flags:
  -h, --help        help for roulette
  -p, --port int    port to listen on (default 8080)
  -r, --recursive   recurse into subdirectories
  -v, --verbose     log accessed files to stdout
      --version     version for roulette

Use "roulette [command] --help" for more information about a command.
```