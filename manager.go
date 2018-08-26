package plantuml

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Error - custom error of this package.
type Error struct {
	// Digested error.
	PackageError error

	// Raw error.
	RawError error
}

// Error returns a digested error text.
func (e *Error) Error() string {
	if e.PackageError != nil {
		return e.PackageError.Error()
	} else if e.RawError != nil {
		return e.RawError.Error()
	} else {
		return ErrInternalError.Error()
	}
}

// newError returns an object of custom error of this package.
func newError(packageError, rawError error) *Error {
	return &Error{packageError, rawError}
}

// SyntaxError - description of syntax error.
type SyntaxError struct {
	// RawError - an error which is returned from PlantUML server.
	RawError string

	// LineNumber - the number of line where syntax error is exists.
	LineNumber int64

	// LineWithError - the raw line with the error.
	LineWithError string
}

// Errors of this package.
var (
	// ErrInternalError - internal error.
	ErrInternalError = errors.New("internal error")

	// ErrServerIsUnavailable - 'server is unavailable' error.
	ErrServerIsUnavailable = errors.New("server is unavailable")

	// ErrInvalidDiagramFormat - 'diagram output format is invalid' error.
	ErrInvalidDiagramFormat = errors.New("diagram output format is invalid")

	// ErrInvalidDiagramDescription - 'diagram description is invalid' error.
	ErrInvalidDiagramDescription = errors.New("there is a syntax error in diagram description or the diagram description is empty")

	// ErrInvalidPlantUMLAddress - invalid PlantUML server address.
	ErrInvalidPlantUMLAddress = errors.New("invalid PlantUML server address")
)

// DiagramFormat - output format of diagram.
type DiagramFormat int

// Available formats of diagrams.
const (
	// DiagramFormatTXT - diagram as txt.
	DiagramFormatTXT DiagramFormat = iota

	// DiagramFormatPNG  - diagram as png.
	DiagramFormatPNG

	// DiagramFormatSVG - diagram as svg.
	DiagramFormatSVG
)

// manager - an implementation of a manager of requests to PlantUML server.
type manager struct {
	// PlantUML server address.
	serverAddr string
}

// Manager of requests to PlantUML server.
type Manager interface {
	// Render returns diagram file in the specified format.
	Render(diagramDescription string, format DiagramFormat) ([]byte, *SyntaxError, error)
}

// NewManager returns client manager object.
func NewManager(plantUMLServerAddr string) (Manager, error) {
	// Validate plantUMLServerAddr.
	_, err := url.ParseRequestURI(plantUMLServerAddr)
	if err != nil {
		return nil, newError(ErrInvalidPlantUMLAddress, err)
	}

	return &manager{serverAddr: plantUMLServerAddr}, nil
}

// Render returns diagram file in the specified format.
func (m *manager) Render(diagramDescription string, format DiagramFormat) ([]byte, *SyntaxError, error) {
	// Validate param 'diagramDescription'.
	diagramDescription = strings.Trim(diagramDescription, " ")
	if len(diagramDescription) == 0 {
		return nil, nil, newError(ErrInvalidDiagramDescription, nil)
	}

	// Validate param 'format'.
	var formatURLPart string
	switch format {
	case DiagramFormatTXT:
		formatURLPart = "txt"
	case DiagramFormatPNG:
		formatURLPart = "png"
	case DiagramFormatSVG:
		formatURLPart = "svg"
	default:
		return nil, nil, newError(ErrInvalidDiagramFormat, nil)
	}

	// 1. Get rendered diagram ID.
	link := fmt.Sprintf("%s/form", m.serverAddr)
	imgID, err := getDiagramID(link, diagramDescription)
	if err != nil {
		return nil, nil, err
	}

	// 2. Get the diagram as TXT to check an error existence.
	link = fmt.Sprintf("%s/txt/%s", m.serverAddr, imgID)
	diagramFile, hasSyntaxError, err := downloadDiagram(link)
	if err != nil {
		return nil, nil, err
	}

	// 3. Check the error if needed.
	if hasSyntaxError {
		syntaxError := getErrorLineNumber(string(diagramFile))
		if syntaxError != nil {
			return nil, syntaxError, newError(ErrInvalidDiagramDescription, nil)
		}
	}

	// 4. Render the diagram in a requred format.
	if format == DiagramFormatTXT {
		return diagramFile, nil, nil
	}
	link = fmt.Sprintf("%s/%s/%s", m.serverAddr, formatURLPart, imgID)
	diagramFile, _, err = downloadDiagram(link)
	if err != nil {
		return nil, nil, err
	}

	return diagramFile, nil, nil
}

// getDiagramID returns ID of the rendered diagram.
func getDiagramID(link, diagramDescription string) (string, error) {
	resp, err := http.PostForm(link, url.Values{"text": {diagramDescription}})
	if err != nil {
		return "", newError(ErrInternalError, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 400 {
		return "", newError(ErrServerIsUnavailable, err)
	}

	urlParts := strings.Split(resp.Request.URL.String(), "/")
	imgID := urlParts[len(urlParts)-1]
	return imgID, nil
}

// downloadDiagram returns diagram and 'has syntax error' flag.
func downloadDiagram(link string) ([]byte, bool, error) {
	resp, err := http.Get(link)
	if err != nil {
		return nil, false, newError(ErrInternalError, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 400 {
		return nil, false, newError(ErrServerIsUnavailable, nil)
	}
	diagramFile, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, false, newError(ErrInternalError, err)
	}
	if resp.StatusCode == 400 {
		return diagramFile, true, nil
	}
	return diagramFile, false, nil
}

// getErrorLineNumber returns an object which describes a syntax error.
//
// It's consider, that an error exists when the diagram (in TXT format) contains
// a substring "[From string (line " in the first line.
func getErrorLineNumber(diagramAsTXT string) *SyntaxError {
	lines := strings.Split(diagramAsTXT, "\n")
	firstLine := lines[0]
	lastLine := lines[len(lines)-1]

	if ok := strings.HasPrefix(firstLine, "[From string (line "); !ok {
		return nil
	}

	lastLine = strings.TrimLeft(lastLine, " Syntax error: ")
	firstLine = strings.TrimLeft(firstLine, "[From string (line ")
	firstLine = strings.TrimRight(firstLine, ") ]")
	lineNumber, err := strconv.Atoi(firstLine)
	if err != nil {
		return &SyntaxError{
			LineNumber:    0,
			LineWithError: lastLine,
			RawError:      diagramAsTXT,
		}
	}

	return &SyntaxError{
		LineNumber:    int64(lineNumber),
		LineWithError: lastLine,
		RawError:      diagramAsTXT,
	}
}
