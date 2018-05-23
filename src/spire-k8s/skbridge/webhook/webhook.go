package webhook

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"text/template"

	admv1b1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

var patchType admv1b1.PatchType = admv1b1.PatchTypeJSONPatch

const (
	containerSpecTmpl = `{
      "image": "{{.SidecarImage}}",
      "name": "spire-sidecar",
      "volumeMounts": [
        {
          "mountPath": "/spire",
          "name": "spire-wl-api"
        }
      ]
  }`

	volumeSpecTmpl = `{
     "hostPath": {
       "path": "{{.HostDir}}",
       "type": "Directory"
     },
     "name": "spire-wl-api"
  }`
)

var (
	cst = template.Must(template.New("container spec").Parse(containerSpecTmpl))
	vst = template.Must(template.New("volume spec").Parse(volumeSpecTmpl))
)

type rfc6902PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

type Config struct {
	Port         string
	CertFilePath string
	KeyFilePath  string
	SidecarImage string
	HostMount    string
}

type webhook struct {
	Config
}

func (wh *webhook) getPatch() ([]byte, error) {
	var containerSpec, volumeSpec bytes.Buffer
	err := cst.Execute(&containerSpec, struct{ SidecarImage string }{wh.SidecarImage})
	if err != nil {
		return nil, err
	}
	err = vst.Execute(&volumeSpec, struct{ HostDir string }{wh.HostMount})
	if err != nil {
		return nil, err
	}

	var cs, vs interface{}
	if err := json.Unmarshal(containerSpec.Bytes(), &cs); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(volumeSpec.Bytes(), &vs); err != nil {
		return nil, err
	}

	sidecarOp := rfc6902PatchOperation{
		Op:    "add",
		Path:  "/spec/containers/-",
		Value: cs,
	}

	volumeOp := rfc6902PatchOperation{
		Op:    "add",
		Path:  "/spec/volumes/-",
		Value: vs,
	}

	return json.Marshal([]rfc6902PatchOperation{sidecarOp, volumeOp})
}

func needsSidecar(pod *corev1.Pod) bool {
	// possible checks based on labels or other criteria
	return true
}

func (wh *webhook) injectServer(w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "Error reading body", http.StatusBadRequest)
		return
	}

	var admRev admv1b1.AdmissionReview
	err = json.Unmarshal(body, &admRev)
	if err != nil {
		log.Printf("Error unmarshaling body: %v", err)
		http.Error(w, "Error unmarshaling body", http.StatusBadRequest)
		return
	}

	admReq := admRev.Request
	if admReq == nil {
		log.Printf("Error unmarshaling body: empty request")
		http.Error(w, "Error unmarshaling body: empty request", http.StatusBadRequest)
		return
	}

	var pod corev1.Pod
	err = json.Unmarshal(admReq.Object.Raw, &pod)
	if err != nil {
		log.Printf("Error unmarshal raw object: %v", err)
		http.Error(w, "Error unmarshaling raw object", http.StatusBadRequest)
		return
	}

	// fill AdmissionResponse object
	admResp := admv1b1.AdmissionResponse{}
	admResp.UID = admReq.UID
	admResp.Allowed = true

	if needsSidecar(&pod) {
		p, err := wh.getPatch()
		if err != nil {
			log.Printf("Error getting patch: %v")
			http.Error(w, "Error getting patch", http.StatusBadRequest)
			return
		}
		admResp.PatchType = &patchType
		admResp.Patch = p
	}

	rev := admv1b1.AdmissionReview{}
	rev.Response = &admResp

	// form HTTP response
	out, err := json.Marshal(rev)
	if err != nil {
		log.Printf("Error marshaling response: %v")
		http.Error(w, "Error marshaling response patch", http.StatusBadRequest)
		return
	}

	w.Write(out)
}

func Start(c Config) error {

	wh := &webhook{
		Config: c,
	}

	http.HandleFunc("/inject", wh.injectServer)
	return http.ListenAndServeTLS(":"+c.Port, c.CertFilePath, c.KeyFilePath, nil)
}
