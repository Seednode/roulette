/*
Copyright © 2025 Seednode <seednode@seedno.de>
*/

package main

import (
	"fmt"
	"log"
	"math"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	AllowedCharacters string = `^[A-z0-9.\-_]+$`
	ReleaseVersion    string = "12.0.0"
)

var (
	AdminPrefix   string
	All           bool
	AllowEmpty    bool
	API           bool
	Audio         bool
	Bind          string
	Code          bool
	CodeTheme     string
	Concurrency   int
	Debug         bool
	ErrorExit     bool
	Fallback      bool
	Flash         bool
	Fun           bool
	Ignore        string
	Images        bool
	Index         bool
	IndexFile     string
	IndexInterval string
	MaxFiles      int
	MinFiles      int
	NoButtons     bool
	Override      string
	Port          int
	Prefix        string
	Profile       bool
	Recursive     bool
	Refresh       bool
	Russian       bool
	Sorting       bool
	Text          bool
	Verbose       bool
	Version       bool
	Videos        bool

	RequiredArgs = []string{
		"all",
		"audio",
		"code",
		"fallback",
		"flash",
		"images",
		"text",
		"video",
	}
)

func main() {
	cmd := &cobra.Command{
		Use:   "roulette <path> [path]...",
		Short: "Serves random media from the specified directories.",
		Args:  cobra.MinimumNArgs(1),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			initializeConfig(cmd)
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case MaxFiles < 0 || MinFiles < 0 || MaxFiles > math.MaxInt32 || MinFiles > math.MaxInt32:
				return ErrInvalidFileCountValue
			case MinFiles > MaxFiles:
				return ErrInvalidFileCountRange
			case Port < 1 || Port > 65535:
				return ErrInvalidPort
			case Concurrency < 1:
				return ErrInvalidConcurrency
			case Ignore != "" && !regexp.MustCompile(AllowedCharacters).MatchString(Ignore):
				return ErrInvalidIgnoreFile
			case Override != "" && !regexp.MustCompile(AllowedCharacters).MatchString(Override):
				return ErrInvalidOverrideFile
			case AdminPrefix != "" && !regexp.MustCompile(AllowedCharacters).MatchString(AdminPrefix):
				return ErrInvalidAdminPrefix
			case AdminPrefix != "":
				AdminPrefix = "/" + AdminPrefix
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return ServePage(args)
		},
	}

	cmd.Flags().StringVar(&AdminPrefix, "admin-prefix", "", "string to prepend to administrative paths")
	cmd.Flags().BoolVarP(&All, "all", "a", false, "enable all supported file types")
	cmd.Flags().BoolVar(&AllowEmpty, "allow-empty", false, "allow specifying paths containing no supported files")
	cmd.Flags().BoolVar(&API, "api", false, "expose REST API")
	cmd.Flags().BoolVar(&Audio, "audio", false, "enable support for audio files")
	cmd.Flags().StringVarP(&Bind, "bind", "b", "0.0.0.0", "address to bind to")
	cmd.Flags().BoolVar(&Code, "code", false, "enable support for source code files")
	cmd.Flags().StringVar(&CodeTheme, "code-theme", "solarized-dark256", "theme for source code syntax highlighting")
	cmd.Flags().IntVar(&Concurrency, "concurrency", 1024, "maximum concurrency for scan threads")
	cmd.Flags().BoolVarP(&Debug, "debug", "d", false, "log file permission errors instead of simply skipping the files")
	cmd.Flags().BoolVar(&ErrorExit, "error-exit", false, "shut down webserver on error, instead of just printing error")
	cmd.Flags().BoolVar(&Fallback, "fallback", false, "serve files as application/octet-stream if no matching format is registered")
	cmd.Flags().BoolVar(&Flash, "flash", false, "enable support for shockwave flash files (via ruffle.rs)")
	cmd.Flags().BoolVar(&Fun, "fun", false, "add a bit of excitement to your day")
	cmd.Flags().StringVar(&Ignore, "ignore", "", "filename used to indicate directory should be skipped")
	cmd.Flags().BoolVar(&Images, "images", false, "enable support for image files")
	cmd.Flags().BoolVarP(&Index, "index", "i", false, "generate index of supported file paths at startup")
	cmd.Flags().StringVar(&IndexFile, "index-file", "", "path to optional persistent index file")
	cmd.Flags().StringVar(&IndexInterval, "index-interval", "", "interval at which to regenerate index (e.g. \"5m\" or \"1h\")")
	cmd.Flags().IntVar(&MaxFiles, "max-files", math.MaxInt32, "skip directories with file counts above this value")
	cmd.Flags().IntVar(&MinFiles, "min-files", 0, "skip directories with file counts below this value")
	cmd.Flags().BoolVar(&NoButtons, "no-buttons", false, "disable first/prev/next/last buttons")
	cmd.Flags().StringVar(&Override, "override", "", "filename used to indicate directory should be scanned no matter what")
	cmd.Flags().IntVarP(&Port, "port", "p", 8080, "port to listen on")
	cmd.Flags().StringVar(&Prefix, "prefix", "/", "root path for http handlers (for reverse proxying)")
	cmd.Flags().BoolVar(&Profile, "profile", false, "register net/http/pprof handlers")
	cmd.Flags().BoolVarP(&Recursive, "recursive", "r", false, "recurse into subdirectories")
	cmd.Flags().BoolVar(&Refresh, "refresh", false, "enable automatic page refresh via query parameter")
	cmd.Flags().BoolVar(&Russian, "russian", false, "remove selected images after serving")
	cmd.Flags().BoolVarP(&Sorting, "sort", "s", false, "enable sorting")
	cmd.Flags().BoolVar(&Text, "text", false, "enable support for text files")
	cmd.Flags().BoolVarP(&Verbose, "verbose", "v", false, "log accessed files and other information to stdout")
	cmd.Flags().BoolVarP(&Version, "version", "V", false, "display version and exit")
	cmd.Flags().BoolVar(&Videos, "video", false, "enable support for video files")

	cmd.CompletionOptions.HiddenDefaultCmd = true

	cmd.Flags().SetInterspersed(true)

	cmd.MarkFlagsOneRequired(RequiredArgs...)

	cmd.SetHelpCommand(&cobra.Command{
		Hidden: true,
	})

	cmd.SetVersionTemplate("roulette v{{.Version}}\n")

	cmd.SilenceErrors = true

	cmd.Version = ReleaseVersion

	log.SetFlags(0)

	err := cmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}

func initializeConfig(cmd *cobra.Command) {
	v := viper.New()

	v.SetEnvPrefix("roulette")

	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	v.AutomaticEnv()

	bindFlags(cmd, v)
}

func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		configName := strings.ReplaceAll(f.Name, "-", "_")

		if !f.Changed && v.IsSet(configName) {
			val := v.Get(configName)
			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}
