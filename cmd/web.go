/*
Copyright © 2022 Seednode <seednode@seedno.de>
*/

package cmd

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func generateHeader() string {
	htmlHeader := `<html>
  <head>
    <title>OUI Lookup</title>
  </head>
  <body>
`

	return htmlHeader
}

func generateFooter() string {
	htmlFooter := `  </body>
</html>`

	return htmlFooter
}

func generatePageRaw(w http.ResponseWriter, output []string) {
	w.Header().Add("Content-Type", "text/plain")

	for i := 0; i < len(output); i++ {
		if Verbose {
			fmt.Println(output[i])
		}

		_, err := io.WriteString(w, output[i]+"\n")
		if err != nil {
			fmt.Println(err)
		}
	}
}

func generatePageHtml(w http.ResponseWriter, output []string) {
	w.Header().Add("Content-Type", "text/html")

	_, err := io.WriteString(w, generateHeader())
	if err != nil {
		fmt.Println(err)
	}

	for i := 0; i < len(output); i++ {
		if Verbose {
			fmt.Println(output[i])
		}

		_, err = io.WriteString(w, "    "+output[i]+"<br />\n")
		if err != nil {
			fmt.Println(err)
		}
	}

	_, err = io.WriteString(w, generateFooter())
	if err != nil {
		fmt.Println(err)
	}
}

func generatePageHelp(r *http.Request, w http.ResponseWriter) {
	w.Header().Add("Content-Type", "text/html")

	_, err := io.WriteString(w, generateHeader())
	if err != nil {
		fmt.Println(err)
	}

	exampleHtmlUrl := fmt.Sprintf("http://%v/7C:0E:CE:FE:FE:FE,10:FE:ED:AB:AB:AB", r.Host)
	exampleRawUrl := fmt.Sprintf("http://%v/raw/7C:0E:CE:FE:FE:FE,10:FE:ED:AB:AB:AB", r.Host)
	exampleJsonUrl := fmt.Sprintf("http://%v/json/7C:0E:CE:FE:FE:FE,10:FE:ED:AB:AB:AB", r.Host)

	help := fmt.Sprintf("    Provide one or more MAC addresses, separated by commas.<br />\n    For example: <a href=%q>%v</a>", exampleHtmlUrl, exampleHtmlUrl)
	help += fmt.Sprintf("<br /><br />\n    Also supports plain text responses.<br />\n    For example: <a href=%q>%v</a>", exampleRawUrl, exampleRawUrl)
	help += fmt.Sprintf("<br /><br />\n    And JSON responses, of course.<br />\n    For example: <a href=%q>%v</a>", exampleJsonUrl, exampleJsonUrl)

	_, err = io.WriteString(w, help)
	if err != nil {
		fmt.Println(err)
	}

	_, err = io.WriteString(w, generateFooter())
	if err != nil {
		fmt.Println(err)
	}
}

func servePageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var ouis []string

		oui := r.RequestURI

		if oui == "" {
			generatePageHelp(r, w)

			return
		}

		args := strings.Split(oui, ",")
		for i := 0; i < len(args); i++ {
			ouis = append(ouis, args[i])
		}

		var output []string

		generatePageHtml(w, output)
	}
}

func doNothing(http.ResponseWriter, *http.Request) {}

func ServePage() {
	defer HandleExit()

	http.HandleFunc("/", servePageHandler())
	http.HandleFunc("/favicon.ico", doNothing)

	port := strconv.Itoa(Port)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
