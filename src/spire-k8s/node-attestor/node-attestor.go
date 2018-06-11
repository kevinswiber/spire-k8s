package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	certificates "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	csrPemBlockType          = "CERTIFICATE REQUEST"
	ecPrivateKeyPemBlockType = "EC PRIVATE KEY"
	idDocFileName            = "id-doc"
)

func getKubeClient(kubeConfigOpt, clientCertFilePath, clientKeyFilePath, caCertFilePath *string) (*kubernetes.Clientset, error) {
	kubeConfigFilePath := *kubeConfigOpt
	if kubeConfigFilePath == "" {
		// Try KUBECONFIG env variable
		kubeConfigFilePath = os.Getenv("KUBECONFIG")
		if kubeConfigFilePath == "" {
			// Still no luck, try default (home)
			home := os.Getenv("HOME")
			if home != "" {
				kubeConfigFilePath = path.Join(home, ".kube", "config")
			}
		}
	}

	if kubeConfigFilePath == "" {
		return nil, fmt.Errorf("Error locating Kubernetes cluster config, please use -kubeconfig to provide location")
	}
	log.Printf("Using kubeconfig file %v", kubeConfigFilePath)

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigFilePath)
	if err != nil {
		log.Fatalf("Error accessing Kubernetes cluster config: %v", err)
	}

	config.TLSClientConfig.CertFile = *clientCertFilePath
	config.TLSClientConfig.KeyFile = *clientKeyFilePath
	config.TLSClientConfig.CAFile = *caCertFilePath

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Error creating clientset: %v", err)
	}
	return clientset, nil
}

func genStorePrivateKey(fileName string) (crypto.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("Error generating ECDSA private key: %v", err)
	}
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("Error marshalling ECDSA private key: %v", err)
	}
	pemKey := &pem.Block{
		Type:  ecPrivateKeyPemBlockType,
		Bytes: keyBytes,
	}
	pemBytes := pem.EncodeToMemory(pemKey)
	err = ioutil.WriteFile(fileName, pemBytes, 0600)
	if err != nil {
		return nil, fmt.Errorf("Error writing private key to file %s: %v", fileName, err)
	}
	return key, nil
}

func genCSR(key crypto.PrivateKey, name string) ([]byte, error) {
	csrTemplate := x509.CertificateRequest{
		Subject: pkix.Name{
			// This exact format is required for the CSR to be auto-approved by kube-controller
			// It must also NOT include any DNSNames or IPAddresses
			Organization: []string{"system:nodes"},
			CommonName:   "system:node:" + name,
		},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}
	return x509.CreateCertificateRequest(rand.Reader, &csrTemplate, key)
}

func getCSRObject(csrBytes []byte, name string) *certificates.CertificateSigningRequest {
	pemBlock := pem.Block{
		Type:  csrPemBlockType,
		Bytes: csrBytes,
	}
	pemBytes := pem.EncodeToMemory(&pemBlock)

	ret := &certificates.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: certificates.CertificateSigningRequestSpec{
			Groups:  []string{"system:authenticated"},
			Request: pemBytes,
			Usages:  []certificates.KeyUsage{"digital signature", "key encipherment", "client auth"},
		},
	}
	return ret
}

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	kubeConfigFilePath := fs.String("kubeconfig", "", "absolute path to the kubeconfig file")
	agentName := fs.String("agent-name", "", "Node agent name to be inserted in the identity document")
	caCertFilePath := fs.String("ca-cert", "", "path to the file containing CA certificate to vallidate the Kubernetes ApiServer certificate")
	clientCertFilePath := fs.String("client-cert", "", "path to the file containing client certificate to authenticate to the Kubernetes ApiServer")
	clientKeyFilePath := fs.String("client-key", "", "path to the file containing the private key to authenticate to the Kubernetes ApiServer")
	idDocDir := fs.String("id-dir", "/tmp/spire-agent-id", "path to the directory where to store the identity document and corresponding private key")
	fs.Parse(os.Args[1:])

	if *caCertFilePath == "" || *clientCertFilePath == "" || *clientKeyFilePath == "" {
		log.Printf("All authentication parameters are required")
		fs.Usage()
		os.Exit(2)
	}

	if *agentName == "" {
		name, err := os.Hostname()
		if err != nil {
			log.Fatalf("Unable to determine agent name automatically, please specify it with -agent-name")
		} else {
			log.Printf("Using agent name %v for the identity document\n", name)
		}
		*agentName = name
	}

	err := os.MkdirAll(*idDocDir, 0700)
	if err != nil {
		log.Fatalf("Error creating identity doc dir %v: %v", *idDocDir, err)
	} else {
		log.Printf("Using identity doc dir: %v", *idDocDir)
	}

	kubeClient, err := getKubeClient(kubeConfigFilePath, clientCertFilePath, clientKeyFilePath, caCertFilePath)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	keyFile := path.Join(*idDocDir, idDocFileName+".key")
	key, err := genStorePrivateKey(keyFile)
	if err != nil {
		log.Fatalf("Error creating private key: %v", err)
	}

	csr, err := genCSR(key, *agentName)
	if err != nil {
		log.Fatalf("Error creating certificate signing request: %v", err)
	}

	csrObjectName := fmt.Sprintf("%s-%d", *agentName, time.Now().Unix())
	csrObject := getCSRObject(csr, csrObjectName)

	// Create CSR object in ApiServer
	csrObject, err = kubeClient.CertificatesV1beta1().CertificateSigningRequests().Create(csrObject)
	if err != nil {
		log.Fatalf("unable to create the certificate signing request: %s", err)
	}

	// Watch for approval
	var cert []byte
	watcher, err := kubeClient.CertificatesV1beta1().CertificateSigningRequests().Watch(metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error watching CSRs on ApiServer: %v", err)
	}
	wc := watcher.ResultChan()
	for event := range wc {
		csr, ok := event.Object.(*certificates.CertificateSigningRequest)
		if !ok {
			log.Fatal("unexpected event type during watch")
		}
		if event.Type == watch.Modified && csr.ObjectMeta.Name == csrObjectName {
			for _, cond := range csr.Status.Conditions {
				if cond.Type == certificates.CertificateApproved {
					cert = csr.Status.Certificate
					break
				}
			}
		}
		if len(cert) > 0 {
			break // approved
		}
	}

	certFile := path.Join(*idDocDir, idDocFileName+".crt")
	if err := ioutil.WriteFile(certFile, cert, 0644); err != nil {
		log.Fatalf("Error writing identitfy document to %s: %s", certFile, err)
	}
	log.Printf("Wrote identity document to %s", certFile)
}
