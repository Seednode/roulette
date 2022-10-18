## About

Sometimes, you just need a way to randomly display images from your filesystem.

Simply point this tool at one or more directories, and then open the specified port (default `8080`) in your browser.

A new image will be selected if you open `/` directly, or if you click on any displayed image.

Browser history is preserved, so you can always go back to any previously displayed image.

Supported file types and extensions are `jp[e]g`, `png`, `gif`, and `webp`.

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
  -h, --help          help for roulette
  -p, --port uint16   port to listen on (default 8080)
  -r, --recursive     recurse into subdirectories
  -v, --verbose       log accessed files to stdout

Use "roulette [command] --help" for more information about a command.
```
