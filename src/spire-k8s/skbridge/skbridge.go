package main

import (
	"flag"
	"log"
	"os"

	"spire-k8s/skbridge/webhook"
)

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	// webhook-related parameters
	whCertFilePath := fs.String("wh-cert", "", "path to the file containing certificate to authenticate to the Kubernetes WebHook")
	whKeyFilePath := fs.String("wh-key", "", "path to the file containing the private key to authenticate to the Kubernetes WebHook")
	whSidecarImage := fs.String("sidecar-image", "", "name of the container image to be injected as sidecar, e.g. nginx:latest")
	whHostMount := fs.String("host-mount", "", "host path mount to be injected in the sidecar container under /spire, e.g. /tmp/spire")
	whPort := fs.String("port", "9999", "TCP port to listen on")
	fs.Parse(os.Args[1:])

	if *whCertFilePath == "" || *whKeyFilePath == "" || *whSidecarImage == "" || *whHostMount == "" {
		fs.Usage()
		os.Exit(2)
	}

	whConfig := webhook.Config{
		Port:         *whPort,
		CertFilePath: *whCertFilePath,
		KeyFilePath:  *whKeyFilePath,
		SidecarImage: *whSidecarImage,
		HostMount:    *whHostMount,
	}

	err := webhook.Start(whConfig)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
