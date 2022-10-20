## About

Sometimes, you just need a way to randomly display images from your filesystem.

Simply point this tool at one or more directories, and then open the specified port (default `8080`) in your browser.

A new image will be selected if you open `/` directly, or if you click on any displayed image.

Browser history is preserved, so you can always go back to any previously displayed image.

Supported file types and extensions are `jp[e]g`, `png`, `gif`, and `webp`.

Builds available [here](https://cdn.seedno.de/builds/roulette).

## Filtering

You can provide a comma-delimited string of patterns to match via the `include=` query parameter.

Only filenames matching one or more of the patterns will be served.

You can also provide a comma-delimited string of patterns to exclude, via the `exclude=` query parameter.

Filenames matching any of these patterns will not be served.

You can combine these two parameters. Exclusions take priority over inclusions.

Both filtering parameters ignore the file extension and full path; they only compare against the bare filename.

## Sorting

You can specify a sorting pattern via the `sort=` query parameter.

A value of `asc` means files will be served in ascending order (lowest-numbered to highest).

If a file exists with a numbered suffix one higher than the currently displayed file, it will be served next.

A value of `desc` means files will be serve in descending order (highest-numbered to lowest).

If a file exists with a numbered suffix one lower than the currently displayed file, it will be served next.

In either case, if no sequential file is found, a new random one will be chosen.

For `asc`, the lowest-numbered file matching a given name will be served first.

For `desc`, the highest-numbered file will be served instead.

These patterns require sequentially-numbered files matching the following pattern: `filename###.extension`.

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
