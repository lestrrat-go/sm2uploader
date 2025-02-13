package main

import (
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	maxMemory = 64 << 20 // 64MB
)

var (
	noTrim           = false
	noShutoff        = false
	noPreheat        = false
	noReinforceTower = false
)

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			log.Printf("Request %s %s completed in %v", r.Method, r.URL.Path, time.Since(start))
		}()
		next.ServeHTTP(w, r)
	})
}

func startOctoPrintServer(listenAddr string, printer *Printer) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"api": "0.1", "server": "1.2.3", "text": "OctoPrint 1.2.3/Dummy"}`))
	})

	mux.HandleFunc("/api/files/local", func(w http.ResponseWriter, r *http.Request) {
		// Check if request is a POST request
		if r.Method != http.MethodPost {
			log.Print("Method not allowed: ", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseMultipartForm(maxMemory); err != nil {
			log.Print("Parse form error: ", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Retrieve the uploaded file
		file, fd, err := r.FormFile("file")
		if err != nil {
			log.Print("Error retrieving file: ", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		// read X-Api-Key header
		apiKey := r.Header.Get("X-Api-Key")
		if len(apiKey) > 5 {
			noTrim = strings.Contains(apiKey, "notrim")
			noPreheat = strings.Contains(apiKey, "nopreheat")
			noShutoff = strings.Contains(apiKey, "noshutoff")
			noReinforceTower = strings.Contains(apiKey, "noreinforcetower")
			msg := make([]string, 0, 3)
			if noTrim {
				msg = append(msg, "-notrim")
			}
			if noPreheat {
				msg = append(msg, "-nopreheat")
			}
			if noShutoff {
				msg = append(msg, "-noshutoff")
			}
			if noReinforceTower {
				msg = append(msg, "-noreinforcetower")
			}
			if len(msg) > 0 {
				log.Printf("SMFix with args: %s", strings.Join(msg, " "))
			}
		}

		// Send the stream to the printer
		payload := NewPayload(file, fd.Filename, fd.Size)
		if err := Connector.Upload(printer, payload); err != nil {
			log.Print("Error uploading file: ", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Upload finished: %s [%s]", fd.Filename, payload.ReadableSize())

		// Return success response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"done": true}`))
	})

	handler := LoggingMiddleware(mux)

	log.Printf("Starting OctoPrint server on %s ...", listenAddr)

	// Create a listener
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	log.Printf("Server started, now you can upload files to http://%s", listener.Addr().String())

	// Start the server
	return http.Serve(listener, handler)
}
