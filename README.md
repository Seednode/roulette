## About

Sometimes, you just need a way to randomly display your files in the browser.

Simply point this tool at one or more directories, and then open the specified port (default 8080) in your browser.

A new file will be selected if you open the root URI directly, or if you click on any displayed image.

Browser history is preserved, so you can always go back to any previously displayed image.

Builds available [here](https://cdn.seedno.de/builds/roulette).

## Usage output
```
Usage:
  roulette <path> [path2]... [flags]
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

Use "roulette [command] --help" for more information about a command.
```