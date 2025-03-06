## About
Sometimes, you just need a way to randomly display media from your filesystem.

Simply point this tool at one or more directories, and then open the specified port (default `8080`) in your browser.

A new file will be selected if you open `/` directly, or if you click on any displayed files.

Browser history is preserved, so you can always go back to any previously displayed media.

Feature requests, code criticism, bug reports, general chit-chat, and unrelated angst accepted at `roulette@seedno.de`.

Static binary builds available [here](https://cdn.seedno.de/builds/roulette).

I only test the linux/amd64, linux/arm64, and windows/amd64 builds, the rest are all best-effort™.

Dockerfile available [here](https://raw.githubusercontent.com/Seednode/roulette/master/docker/Dockerfile).

An example instance with most features enabled can be found [here](https://cdn.seedno.de/random).

### Configuration
The following configuration methods are accepted, in order of highest to lowest priority:
- Command-line flags
- Environment variables

## Admin prefix
You can restrict access to certain functionality (the REST API and profiling endpoints) by prepending a secret string to the paths.

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
- `/index/rebuild`
- `/types/available`
- `/types/enabled`

While this might thwart very basic attacks, the proper solution for most use cases would likely be to add authentication via a reverse proxy.

## API
If the `--api` flag is passed, a number of REST endpoints are registered.

The first—`/index/rebuild`—responds to POST requests by rebuilding the index.

This can prove useful when confirming whether the index is generated successfully, or whether a given file is in the index.

The remaining four endpoints respond to GET requests with information about the registered file types:
- `/extensions/available`
- `/extensions/enabled`
- `/types/available`
- `/types/enabled`

## Ignoring directories
If the `--ignore <filename>` flag is passed, any directory containing a file with the specified name will be skipped during the scanning stage.

## Indexing
If the `-i|--index` flag is passed, all specified paths will be indexed on start.

This will slightly increase the delay before the application begins responding to requests, but should significantly speed up subsequent requests.

Automatic index rebuilds can be enabled via the `--index-interval <duration>` flag, which accepts [time.Duration](https://pkg.go.dev/time#ParseDuration) strings.

If `--index-file <filename>` is set, the index will be loaded from the specified file on start, and written to the file whenever it is re-generated.

The index file consists of [zstd](https://facebook.github.io/zstd/)-compressed [gobs](https://pkg.go.dev/encoding/gob).

## Refresh
If the `--refresh` flag is passed and a positive-value `refresh=<integer><unit>` query parameter is provided, the page will reload after that interval.

This can be used to generate a sort of slideshow of files in any browser with Javascript support.

Pressing Spacebar will pause automatic refreshing until Spacebar is pressed again, the page is manually refreshed, or a new page is loaded.

Minimum accepted value is 500ms, as anything lower seems to cause inconsistent behavior. This might be changed in a future release.

Supported units are `ns`, `us`/`µs`, `ms`, `s`, `m`, and `h`.

## Russian
If the `--russian` flag is passed, everything functions exactly as you would expect.

That is, files will be deleted after being served. This is not a joke, you *will* lose data.

This uses [os.Remove()](https://pkg.go.dev/os#Remove) and checks to ensure the specified file is inside one of the paths passed to `roulette`.

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

Any [supported theme](https://pkg.go.dev/github.com/alecthomas/chroma/v2@v2.9.1/styles#pkg-variables) can be passed via the `--code-theme` flag.

By default, [`solarized-dark256`](https://xyproto.github.io/splash/docs/solarized-dark256.html) is used.

### Environment variables
Almost all options configurable via flags can also be configured via environment variables. 

The associated environment variable is the prefix `ROULETTE_` plus the flag name, with the following changes:
- Leading hyphens removed
- Converted to upper-case
- All internal hyphens converted to underscores

For example:
- `--admin-prefix /test/` becomes `ROULETTE_ADMIN_PREFIX=/test/`
- `--index-file ~/index.zstd` becomes `ROULETTE_INDEX_FILE=~/index.zstd`
- `--images` becomes `ROULETTE_IMAGES=true`

## Usage output
```
Serves random media from the specified directories.

Usage:
  roulette <path> [path]... [flags]

Flags:
      --admin-prefix string     string to prepend to administrative paths
  -a, --all                     enable all supported file types
      --allow-empty             allow specifying paths containing no supported files
      --api                     expose REST API
      --audio                   enable support for audio files
  -b, --bind string             address to bind to (default "0.0.0.0")
      --code                    enable support for source code files
      --code-theme string       theme for source code syntax highlighting (default "solarized-dark256")
      --concurrency int         maximum concurrency for scan threads (default 1024)
  -d, --debug                   log file permission errors instead of simply skipping the files
      --error-exit              shut down webserver on error, instead of just printing error
      --fallback                serve files as application/octet-stream if no matching format is registered
      --flash                   enable support for shockwave flash files (via ruffle.rs)
      --fun                     add a bit of excitement to your day
  -h, --help                    help for roulette
      --ignore string           filename used to indicate directory should be skipped
      --images                  enable support for image files
  -i, --index                   generate index of supported file paths at startup
      --index-file string       path to optional persistent index file
      --index-interval string   interval at which to regenerate index (e.g. "5m" or "1h")
      --max-files int           skip directories with file counts above this value (default 2147483647)
      --min-files int           skip directories with file counts below this value
      --no-buttons              disable first/prev/next/last buttons
      --override string         filename used to indicate directory should be scanned no matter what
  -p, --port int                port to listen on (default 8080)
      --prefix string           root path for http handlers (for reverse proxying) (default "/")
      --profile                 register net/http/pprof handlers
  -r, --recursive               recurse into subdirectories
      --refresh                 enable automatic page refresh via query parameter
      --russian                 remove selected images after serving
  -s, --sort                    enable sorting
      --text                    enable support for text files
      --tls-cert string         path to TLS certificate
      --tls-key string          path to TLS keyfile
  -v, --verbose                 log accessed files and other information to stdout
  -V, --version                 display version and exit
      --video                   enable support for video files
```

## Building the Docker image
From inside the cloned repository, build the image using the following command:

`REGISTRY=<registry url> LATEST=yes TAG=alpine ./build-docker.sh`
