/*
Copyright 2014 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// outyet is a web server that announces whether or not a particular Go version
// has been tagged.
package main

import (
	"expvar"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Command-line flags.
var (
	httpAddr   = flag.String("http", ":8088", "Listen address")
	pollPeriod = flag.Duration("poll", 5*time.Second, "Poll period")
	version    = flag.String("version", "1.22.0", "Go version")
)

const baseChangeURL = "https://go.googlesource.com/go/+/"

func main() {
	flag.Parse()
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	changeURL := fmt.Sprintf("%sgo%s", baseChangeURL, *version)
	http.Handle("/", NewServer(*version, changeURL, hostname, *pollPeriod, ""))
	http.Handle("/die", NewServer(*version, changeURL, hostname, *pollPeriod, "die"))
	log.Fatal(http.ListenAndServe(*httpAddr, nil))
}

func die() {
	os.Exit(99)
}

// Exported variables for monitoring the server.
// These are exported via HTTP as a JSON object at /debug/vars.
var (
	hitCount       = expvar.NewInt("hitCount")
	pollCount      = expvar.NewInt("pollCount")
	pollError      = expvar.NewString("pollError")
	pollErrorCount = expvar.NewInt("pollErrorCount")
)

// Server implements the outyet server.
// It serves the user interface (it's an http.Handler)
// and polls the remote repository for changes.
type Server struct {
	version        string
	url            string
	hostname       string
	logic          string
	period         time.Duration
	config_message string
	secret_message string

	mu  sync.RWMutex // protects the yes variable
	yes bool
}

// NewServer returns an initialized outyet server.
func NewServer(version, url string, hostname string, period time.Duration, logic string) *Server {
	config_message := "Note: envvar is not set in the environment."
	envvar, exists := os.LookupEnv("envvar")
	if exists != false {
		config_message = fmt.Sprintf("envar is set to: %s", envvar)
	}
	secret_message := "Note: secret is not set in the environment."
	secret, s_exists := os.LookupEnv("secret")
	if s_exists != false {
		length := len(secret)
		redacted_secret := strings.Repeat("*", length)
		secret_message = fmt.Sprintf("secret is set to: %s", redacted_secret)
	}
	s := &Server{version: version, url: url, hostname: hostname, period: period, config_message: config_message, secret_message: secret_message, logic: logic}
	go s.poll()
	return s
}

// poll polls the change URL for the specified period until the tag exists.
// Then it sets the Server's yes field true and exits.
func (s *Server) poll() {
	for !isTagged(s.url) {
		pollSleep(s.period)
	}
	s.mu.Lock()
	s.yes = true
	s.mu.Unlock()
	pollDone()
}

// Hooks that may be overridden for integration tests.
var (
	pollSleep = time.Sleep
	pollDone  = func() {}
)

// isTagged makes an HTTP HEAD request to the given URL and reports whether it
// returned a 200 OK response.
func isTagged(url string) bool {
	pollCount.Add(1)
	r, err := http.Head(url)
	if err != nil {
		log.Print(err)
		pollError.Set(err.Error())
		pollErrorCount.Add(1)
		return false
	}
	return r.StatusCode == http.StatusOK
}

// ServeHTTP implements the HTTP user interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hitCount.Add(1)
	s.mu.RLock()
	data := struct {
		ConfigMessage string
		SecretMessage string
		URL           string
		Version       string
		Hostname      string
		Logic         string
		Yes           bool
	}{
		s.config_message,
		s.secret_message,
		s.url,
		s.version,
		s.hostname,
		s.logic,
		s.yes,
	}
	s.mu.RUnlock()
	if s.logic == "die" {
		die()
	}
	err := tmpl.Execute(w, data)
	if err != nil {
		log.Print(err)
	}
}

// tmpl is the HTML template that drives the user interface.
var tmpl = template.Must(template.New("tmpl").Parse(`
<!DOCTYPE html><html>
	<head></head>
	<body><center>
	<h2>Is Go {{.Version}} out yet?</h2>
	<h1>
	{{if .Yes}}
		<a href="{{.URL}}">YES!</a>
	{{else}}
		No. :-(<br></br>
		But you can see a list of all releases here:<br></br>
		<a href="https://github.com/golang/go/releases">https://github.com/golang/go/releases</a>
	{{end}}
	</h1>
	<p>Hostname: {{.Hostname}}</p>
	<hr>
	<p>{{.ConfigMessage}}<br>
	{{.SecretMessage}}</p>
	<hr>
</center></body></html>
`))
