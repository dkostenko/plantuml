package api

import (
	"encoding/json"
	"log"
	"net/http"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/gorilla/mux"

	"github.com/dkostenko/plantuml"
)

// manager - an implementation of a manager of requests to HTTP API server.
type manager struct {
	// PlantUML client.
	client plantuml.Manager
	router *mux.Router
}

// Manager of requests to HTTP API server.
type Manager interface {
	// Listen starts listening on specified address.
	Listen(addr string) error
}

// NewManager returns manager of requests to HTTP API server.
func NewManager(client plantuml.Manager) Manager {
	m := &manager{
		client: client,
		router: mux.NewRouter(),
	}

	m.router.HandleFunc("/api/render-diagram", m.handlerRenderDiagram).Methods("POST")
	m.router.Handle("/", http.FileServer(
		&assetfs.AssetFS{
			Asset:     Asset,
			AssetDir:  AssetDir,
			AssetInfo: AssetInfo,
		})).Methods("GET")
	m.router.Handle("/{everethig}", http.FileServer(
		&assetfs.AssetFS{
			Asset:     Asset,
			AssetDir:  AssetDir,
			AssetInfo: AssetInfo,
		})).Methods("GET")
	return m
}

// Listen starts listening on specified address.
func (m *manager) Listen(addr string) error {
	log.Printf("Listening on %s", addr)
	return http.ListenAndServe(addr, m.router)
}

// handlerRenderDiagram sends to the client the generated diagram from
// the diagram description in the specified format.
func (m *manager) handlerRenderDiagram(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var prms prmsRenderDiagram
	err := decoder.Decode(&prms)
	if err != nil {
		m.sendErr(w, 2, nil)
		return
	}

	var format plantuml.DiagramFormat
	switch prms.Format {
	case "svg":
		format = plantuml.DiagramFormatSVG
	case "png":
		format = plantuml.DiagramFormatPNG
	case "txt":
		format = plantuml.DiagramFormatTXT
	default:
		m.sendErr(w, 4, nil)
		return
	}

	diagramFile, syntaxErr, err := m.client.Render(prms.Data, format)
	if err != nil {
		if err.(*plantuml.Error).PackageError == plantuml.ErrInvalidDiagramDescription {
			m.sendErr(w, 3, map[string]interface{}{
				"syntax_error_line": syntaxErr.LineNumber,
				"line_with_error":   syntaxErr.LineWithError,
				"raw":               syntaxErr.RawError,
			})
		} else {
			m.sendErr(w, 4, nil)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(diagramFile)
}

// prmsRenderDiagram - body params for 'handlerRenderDiagram'.
type prmsRenderDiagram struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

// sendErr sends to the client a server error in standart wrapper.
func (m *manager) sendErr(w http.ResponseWriter, errorCode int64, errorData interface{}) {
	w.WriteHeader(http.StatusInternalServerError)
	enc := json.NewEncoder(w)
	enc.Encode(map[string]interface{}{
		"ok":         false,
		"error_code": errorCode,
		"error_data": errorData,
	})
}
