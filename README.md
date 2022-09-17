## About

Sometimes, you just need a way to randomly display your files in the browser.

Simply point this tool at one or more directories, and then open the specified port (default 8080) in your browser.

A new file will be selected if you open the root URI directly, or if you click on any displayed image.

Browser history is preserved, so you can always go back to any previously displayed image.

Builds available [here](https://cdn.seedno.de/builds/roulette).

## [-s, --successive]

This option is tailored specifically for my own use case. When loading a new image, it checks for a successively-numbered file in the same path.

For example, if the file `/mnt/photos/MyVacation001.jpg` is being displayed, clicking on the image will search for a `/mnt/photos/MyVacation002.jpg`.

If a matching file is not found, it will select a random file as usual.

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
  -h, --help         help for roulette
  -p, --port int     port to listen on (default 8080)
  -r, --recursive    recurse into subdirectories
  -s, --successive   load the next sequential file, if possible
  -v, --verbose      log accessed files to stdout

Use "roulette [command] --help" for more information about a command.
```