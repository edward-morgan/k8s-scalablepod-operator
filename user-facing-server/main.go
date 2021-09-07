package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

var operatorDnsName string
var operatorPort string
var operatorPath string
var listenerPort string

/* Start up an HTTP server on <PORT> that, when a external request is received, POSTS to OPERATOR_DNS_NAME:OPERATOR_PORT to start a ScalablePod.
 */
func main() {
	operatorDnsName = os.Getenv("OPERATOR_DNS_NAME")
	operatorPort = os.Getenv("OPERATOR_PORT")
	operatorPath = os.Getenv("OPERATOR_PATH")
	listenerPort = os.Getenv("PORT")
	log.Printf("Starting server on localhost:%s.\n", listenerPort)
	http.HandleFunc("/", Handler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", listenerPort), nil))
}

func Handler(w http.ResponseWriter, req *http.Request) {
	log.Println("Received request for ScalablePod.")
	resp, err := http.Post(fmt.Sprintf("http://%s:%s%s", operatorDnsName, operatorPort, operatorPath), "application/text", strings.NewReader("Request"))
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Could not contact operator.\n"))
		return
	}
	switch resp.StatusCode {
	case http.StatusOK:
		log.Println("Received 200 from operator.")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Spinning up ScalablePod...\n"))
	case http.StatusInternalServerError:
		log.Println("Server failed to schedule.")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Spinning up ScalablePod...\n"))
	case http.StatusNotFound:
		log.Println("No resources available to schedule.")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("No resources currently available. Try again later.\n"))
	}
}
