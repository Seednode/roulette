## About

Sometimes, you just need a way to randomly display images from your filesystem.

Simply point this tool at one or more directories, and then open the specified port (default `8080`) in your browser.

A new image will be selected if you open `/` directly, or if you click on any displayed image.

Browser history is preserved, so you can always go back to any previously displayed image.

Supported file types and extensions are `bmp`, `gif`, `jp[e]g`, `png`, and `webp`.

Feature requests, code criticism, bug reports, general chit-chat, and unrelated angst accepted at `roulette@seedno.de`.

Static binary builds available [here](https://cdn.seedno.de/builds/roulette).

I only test the linux/amd64, linux/arm64, and windows/amd64 builds, the rest are all best-effort™.

x86_64 and ARM Docker images of latest version: `oci.seedno.de/seednode/roulette:latest`.

Dockerfile available [here](https://git.seedno.de/seednode/docker-roulette).

## Filtering

You can provide a comma-delimited string of alphanumeric patterns to match via the `include=` query parameter, assuming the `-f|--filter` flag is enabled.

Only filenames matching one or more of the patterns will be served.

You can also provide a comma-delimited string of alphanumeric patterns to exclude, via the `exclude=` query parameter.

Filenames matching any of these patterns will not be served.

You can also combine these two parameters, with exclusions taking priority over inclusions.

Both filtering parameters ignore the file extension and full path; they only compare against the bare filename.

## Sorting

You can specify a sorting pattern via the `sort=` query parameter, assuming the `-s|--sort` flag is enabled.

A value of `sort=asc` means files will be served in ascending order (lowest-numbered to highest).

If a file exists with a numbered suffix one higher than the currently displayed file, it will be served next.

A value of `sort=desc` means files will be served in descending order (highest-numbered to lowest).

If a file exists with a numbered suffix one lower than the currently displayed file, it will be served next.

In either case, if no sequential file is found, a new random one will be chosen.

For `sort=asc`, the lowest-numbered file matching a given name will be served first.

For `sort=desc`, the highest-numbered file will be served instead.

If any other (or no) value is provided, the selected file will be random.

Note: These patterns require sequentially-numbered files matching the following pattern: `filename###.extension`.

## Refresh

If a positive-value `refresh=<integer><unit>` query parameter is provided, the page will reload after that interval.

This can be used to generate a sort of slideshow of images.

Supported units are `ns`, `us`/`µs`, `ms`, `s`, `m`, and `h`.

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
  -c, --cache         only scan directories once, at startup (incompatible with --filter)
  -f, --filter        enable filtering via query parameters (incompatible with --cache)
  -h, --help          help for roulette
  -p, --port uint16   port to listen on (default 8080)
  -r, --recursive     recurse into subdirectories
  -s, --sort          enable sorting via query parameters
  -v, --verbose       log accessed files to stdout

Use "roulette [command] --help" for more information about a command.
```
