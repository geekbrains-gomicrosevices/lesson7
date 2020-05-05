package render

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

func RenderJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	body, err := json.Marshal(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(body)
}

func RenderJSONErr(w http.ResponseWriter, err string, status int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, err)))
}

var TT = make(map[string]*template.Template)
var TTPath = make(map[string]string)
var Layout string
var TemplateDir string

func SetTemplateDir(path string) {
	TemplateDir = path
}

func SetTemplateLayout(path string) {
	Layout = path
}

func AddTemplate(name, path string) {
	TTPath[name] = path
}

func ParseTemplates() (err error) {

	for name, path := range TTPath {
		templatePath := filepath.Join(TemplateDir, path)
		if Layout != "" {
			layoutPath := filepath.Join(TemplateDir, Layout)
			TT[name], err = template.ParseFiles(layoutPath, templatePath)
		} else {
			TT[name], err = template.ParseFiles(templatePath)
		}
		if err != nil {
			return fmt.Errorf("Error while preparing template '%s', path '%s': %w", name, path, err)
		}
	}

	return nil
}

func RenderTemplate(w http.ResponseWriter, name string, data interface{}) {
	var err error
	defer func() {
		if err != nil {
			log.Printf("Render error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()

	t, ok := TT[name]
	if !ok {
		err = fmt.Errorf("No template '%v'", name)
		return
	}

	err = t.Execute(w, data)

	return
}
