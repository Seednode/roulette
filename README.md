## About
Sometimes, you just need a way to randomly display media from your filesystem.

Simply point this tool at one or more directories, and then open the specified port (default `8080`) in your browser.

A new file will be selected if you open `/` directly, or if you click on any displayed files.

Browser history is preserved, so you can always go back to any previously displayed media.

Feature requests, code criticism, bug reports, general chit-chat, and unrelated angst accepted at `roulette@seedno.de`.

Static binary builds available [here](https://cdn.seedno.de/builds/roulette).

I only test the linux/amd64, linux/arm64, and windows/amd64 builds, the rest are all best-effort™.

x86_64 and ARM Docker images of latest version: `oci.seedno.de/seednode/roulette:latest`.

Dockerfile available [here](https://git.seedno.de/seednode/roulette/raw/branch/master/docker/Dockerfile).

An example instance with most features enabled can be found [here](https://nature.seedno.de/).

## Admin prefix
You can restrict access to certain functionality by prepending a secret string to the paths.

For example, providing the `--admin-prefix=abc123` flag will register the index rebuild path as `/abc123/index/rebuild`.

The restricted paths are:
- `/debug/pprof/allocs`
- `/debug/pprof/block`
- `/debug/pprof/cmdline`
- `/debug/pprof/goroutine`
- `/debug/pprof/heap`
- `/debug/pprof/mutex`
- `/debug/pprof/profile`
- `/debug/pprof/symbol`
- `/debug/pprof/threadcreate`
- `/debug/pprof/trace`
- `/extensions/available`
- `/extensions/enabled`
- `/index/`
- `/index/rebuild`
- `/types/available`
- `/types/enabled`

While this might thwart very basic attacks, the proper solution for most use cases would likely be to add authentication via a reverse proxy.

If the `--redact` flag is passed, references to the admin prefix will be redacted in log output.

## Filtering
You can provide a comma-delimited string of alphanumeric patterns to match via the `include=` query parameter, assuming the `-f|--filter` flag is enabled.

Only filenames matching one or more of the patterns will be served.

You can also provide a comma-delimited string of alphanumeric patterns to exclude, via the `exclude=` query parameter.

Filenames matching any of these patterns will not be served.

You can combine these two parameters, with exclusions taking priority over inclusions.

Both filtering parameters ignore the file extension and full path; they only compare against the bare filename.

## Ignoring directories
If the `--ignore` flag is passed, any directory containing a file named `.roulette-ignore` (configurable with `--ignore-file`) will be skipped during the scanning stage.

## Indexing
If the `-i|--indexing` flag is passed, all specified paths will be indexed on start.

This will slightly increase the delay before the application begins responding to requests, but should significantly speed up subsequent requests.

The index can be regenerated at any time by accessing the `/index/rebuild` endpoint.

If `--index-file` is set, the index will be loaded from the specified file on start, and written to the file whenever it is re-generated.

The index file consists of (optionally compressed) [gobs](https://pkg.go.dev/encoding/gob).

The compression format can be specified via the `--compression` flag.

Supported formats are `none`, `zlib`, and `zstd`.

Optionally, `--compression-fast` can be used to use the fastest instead of the best compression mode.

## Info
If the `-i|--info` flag is passed, six additional endpoints are registered.

The first of these—`/index/html` and `/index/json`—return the contents of the index, in HTML and JSON formats respectively. 

If `--page-length` is also set, these can be viewed in paginated form by appending a page number, e.g. `/index/html/5` for the fifth page.

This can prove useful when confirming whether the index is generated successfully, or whether a given file is in the index.

The remaining four endpoints—`/extensions/available`, `/extensions/enabled`, `/types/available` and `/types/enabled`—return information about the registered file types.

## Refresh
If the `--refresh` flag is passed and a positive-value `refresh=<integer><unit>` query parameter is provided, the page will reload after that interval.

This can be used to generate a sort of slideshow of files in any browser with Javascript support.

Pressing Spacebar will pause automatic refreshing until Spacebar is pressed again, the page is manually refreshed, or a new page is loaded.

Minimum accepted value is 500ms, as anything lower seems to cause inconsistent behavior. This might be changed in a future release.

Supported units are `ns`, `us`/`µs`, `ms`, `s`, `m`, and `h`.

## Russian
If the `--russian` flag is passed, everything functions exactly as you would expect.

That is, files will be deleted after being served. This is not a joke, you *will* lose data.

This uses `os.Remove()` and checks to ensure the specified file is inside one of the paths passed to `roulette`.

That said, this has not been tested to any real extent, so only pass this flag on systems you don't care about.

Enjoy!

## Sorting
You can specify a sorting direction via the `sort=` query parameter, assuming the `-s|--sort` flag is enabled.

A value of `sort=asc` means files will be served in ascending order (lowest-numbered to highest).

If a file exists with a numbered suffix one higher than the currently displayed file, it will be served next.

A value of `sort=desc` means files will be served in descending order (highest-numbered to lowest).

If a file exists with a numbered suffix one lower than the currently displayed file, it will be served next.

In either case, if no sequential file is found, a new random one will be chosen.

For `sort=asc`, the lowest-numbered file matching a given name will be served first.

For `sort=desc`, the highest-numbered file will be served instead.

If any other (or no) value is provided, the selected file will be random.

Note: These options require sequentially-numbered files matching the following pattern: `filename[0-9]*.extension`.

## Themes
The `--code` handler provides syntax highlighting via [alecthomas/chroma](https://github.com/alecthomas/chroma).

Any [supported theme](https://pkg.go.dev/github.com/alecthomas/chroma/v2@v2.9.1/styles#pkg-variables) can be passed via the `--theme` flag.

By default, [`solarized-dark256`](https://xyproto.github.io/splash/docs/solarized-dark256.html) is used.

## Usage output
```
Serves random media from the specified directories.

Usage:
  roulette <path> [path]... [flags]

Flags:
      --admin-prefix string   string to prepend to administrative paths
  -a, --all                   enable all supported file types
      --allow-empty           allow specifying paths containing no supported files
      --audio                 enable support for audio files
      --binary-prefix         use IEC binary prefixes instead of SI decimal prefixes
  -b, --bind string           address to bind to (default "0.0.0.0")
      --case-sensitive        use case-sensitive matching for filters
      --code                  enable support for source code files
      --code-theme string     theme for source code syntax highlighting (default "solarized-dark256")
      --concurrency int       maximum concurrency for scan threads (default 2147483647)
  -d, --debug                 display even more verbose logs
      --disable-buttons       disable first/prev/next/last buttons
      --exit-on-error         shut down webserver on error, instead of just printing error
      --fallback              serve files as application/octet-stream if no matching format is registered
  -f, --filter                enable filtering
      --flash                 enable support for shockwave flash files (via ruffle.rs)
      --fun                   add a bit of excitement to your day
      --handlers              display registered handlers (for debugging)
  -h, --help                  help for roulette
      --ignore                skip all directories containing a specified filename
      --ignore-file string    filename used to indicate directory to be skipped (default ".roulette-ignore")
      --images                enable support for image files
      --index                 generate index of supported file paths at startup
      --index-file string     path to optional persistent index file
  -i, --info                  expose informational endpoints
      --max-file-count int    skip directories with file counts above this value (default 2147483647)
      --min-file-count int    skip directories with file counts below this value
  -p, --port int              port to listen on (default 8080)
      --prefix string         root path for http handlers (for reverse proxying) (default "/")
      --profile               register net/http/pprof handlers
  -r, --recursive             recurse into subdirectories
      --redact                redact admin prefix in log output
      --refresh               enable automatic page refresh via query parameter
      --russian               remove selected images after serving
  -s, --sort                  enable sorting
      --text                  enable support for text files
  -v, --verbose               log accessed files and other information to stdout
  -V, --version               display version and exit
      --video                 enable support for video files
```

## Building the Docker container
From inside the `docker/` subdirectory, build the image using the following command:

`REGISTRY=<registry url> LATEST=yes TAG=alpine ./build.sh`
