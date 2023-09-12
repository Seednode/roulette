## About

Sometimes, you just need a way to randomly display media from your filesystem.

Simply point this tool at one or more directories, and then open the specified port (default `8080`) in your browser.

A new file will be selected if you open `/` directly, or if you click on any displayed files.

Browser history is preserved, so you can always go back to any previously displayed media.

Supported file types and extensions are `bmp`, `gif`, `jp[e]g`, `m4a`, `mp3`, `mp4`, `.ogg`, `.ogv`, `png`, `wav`, `webm`, `webp`.

Feature requests, code criticism, bug reports, general chit-chat, and unrelated angst accepted at `roulette@seedno.de`.

Static binary builds available [here](https://cdn.seedno.de/builds/roulette).

I only test the linux/amd64, linux/arm64, and windows/amd64 builds, the rest are all best-effort™.

x86_64 and ARM Docker images of latest version: `oci.seedno.de/seednode/roulette:latest`.

Dockerfile available [here](https://git.seedno.de/seednode/roulette/raw/branch/master/docker/Dockerfile).

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

This can be used to generate a sort of slideshow of files.

Minimum accepted value is 500ms, as anything lower seems to cause inconsistent behavior. This might be changed in a future release.

Supported units are `ns`, `us`/`µs`, `ms`, `s`, `m`, and `h`.

## Caching

If the `-c|--cache` flag is passed, the indices of all specified paths will be cached on start.

This will slightly increase the delay before the application begins responding to requests, but should significantly speed up subsequent requests.

If any `include=`/`exclude=` filters are specified in a given request, the cache will be bypassed for that specific request.

The cache can be regenerated at any time by accessing the `/clear_cache` endpoint.

If `--cache-file` is set, the cache will be loaded from the specified file on start, and written to the file whenever it is re-generated.

## Statistics

If the `--stats` flag is passed, an additional endpoint, `/stats`, is registered.

When accessed, this endpoint returns a JSON document listing every file served, along with the number of times it has been served, its filesize, and timestamps of when it was served.

## Debug
If the `-d|--debug` flag is passed, two additional endpoints—`/html` and `/json`—are registered.

When accessed, these endpoints return the contents of the index, in HTML and JSON formats respectively. This can prove useful when confirming whether the index is generated successfully, or whether a given file is in the index.

## Russian
If the `--russian` flag is passed, everything functions exactly as you would expect.

That is, files will be deleted after being served. This is not a joke, you *will* lose data.

This uses `os.Remove()` and checks to ensure the specified file is inside one of the paths passed to `roulette`.

That said, this has not been tested to any real extent, so only pass this flag on systems you don't care about.

Enjoy!

## Usage output
```
Serves random media from the specified directories.

Usage:
  roulette <path> [path]... [flags]

Flags:
      --all                       enable all supported file types
      --audio                     enable support for audio files
  -b, --bind string               address to bind to (default "0.0.0.0")
  -c, --cache                     generate directory cache at startup
      --cache-file string         path to optional persistent cache file
  -d, --debug                     expose debug endpoint
  -f, --filter                    enable filtering
  -h, --help                      help for roulette
      --images                    enable support for image files (default true)
      --maximum-files uint32      skip directories with file counts over this value (default 4294967295)
      --minimum-files uint32      skip directories with file counts under this value
      --page-length uint16        pagination length for statistics and debug pages
  -p, --port uint16               port to listen on (default 8080)
      --profile                   register net/http/pprof handlers
  -r, --recursive                 recurse into subdirectories
      --refresh-interval string   force refresh interval equal to this duration (minimum 500ms)
      --russian                   remove selected images after serving
  -s, --sort                      enable sorting
      --stats                     expose stats endpoint
      --stats-file string         path to optional persistent stats file
      --text                      enable support for text files (default true)
  -v, --verbose                   log accessed files to stdout
  -V, --version                   display version and exit
      --video                     enable support for video files
```

## Building the Docker container
From inside the `docker/` subdirectory, build the image using the following command:

`REGISTRY=<registry url> LATEST=yes TAG=alpine ./build.sh`
