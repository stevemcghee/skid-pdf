package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/schema"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// http://localhost:8080/pdf?grayscale=false&landscape=true&uri=developers.mindbodyonline.com
func pdfHandle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	switch r.Method {
	case "POST":
		pdfr := pdfRequest{}

		if r.Body == nil {
			http.Error(w, "Please send a request body", 400)
			return
		}
		err := json.NewDecoder(r.Body).Decode(&pdfr)
		if err != nil {
			http.Error(w, err.Error(), 400)
			httpReqs.WithLabelValues("400", "POST").Inc()
			return
		}
		decoder := schema.NewDecoder()
		// r.PostForm is a map of our POST form values
		err = decoder.Decode(&pdfr, r.PostForm)

		if err != nil || pdfr.URL == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		result := generateFromPDFRequest(&pdfr)
		w.Header().Set("Content-Type", "application/pdf")
		httpReqs.WithLabelValues("200", "POST").Inc()
		w.Write(result)

	case "GET":
		// simple
		r.ParseForm()

		grayscaleForm := r.Form.Get("grayscale")
		useGrayscal, err := strconv.ParseBool(grayscaleForm)
		if err != nil {
			fmt.Println("Unable to parse grayscale from query string")
			fmt.Println(err)
		}

		landscapeForm := r.Form.Get("landscape")
		useLandscape, err := strconv.ParseBool(landscapeForm)
		if err != nil {
			fmt.Println("Unable to parse landscape from query string")
			fmt.Println(err)
		}

		pdfURL := fmt.Sprintf("%s", r.Form.Get("uri")) //, r.Form.Get("sid"))
		if pdfURL == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		extraParams := []string{}
		if useGrayscal {
			extraParams = append(extraParams, WkGrayscale...)
		}
		if useLandscape {
			extraParams = append(extraParams, WkOrientationLandscape...)
		}

		result := generateWKPDF(pdfURL, extraParams)
		w.Header().Set("Content-Type", "application/pdf")
		duration := time.Since(start)
		// instrumentation:
		pdfTime.WithLabelValues(fmt.Sprintf("%d", 200)).Observe(duration.Seconds())
		httpReqs.WithLabelValues("200", "GET").Inc()
		w.Write(result)
	}
}

func source(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://github.com/vacoj/skid-pdf", 301)
}

func help(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://github.com/vacoj/skid-pdf/wiki", 301)
}

func webRoot(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/index.html")
}

func startHTTPListener() {
	http.HandleFunc("/", webRoot)   // a little web form for making PDFs
	http.HandleFunc("/src", source) // exlains what the service does and how to use it.
	http.HandleFunc("/help", help)  // goes to source page
	http.HandleFunc("/pdf", pdfHandle)

	// start prometheus export for monitoring
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(promAddr, nil))

	// static content
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, r.URL.Path[1:])
		httpReqs.WithLabelValues("200", "GET", "static").Inc()
	})

	s := &http.Server{
		Addr:           ":" + settings.HTTPPort,
		ReadTimeout:    100 * time.Second,
		WriteTimeout:   100 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Panic(s.ListenAndServe())
}
