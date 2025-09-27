package core

import (
	"html/template"
)

// A struct to hold the data for a template.
type Index struct {
	Title         string
	BaseURL       string
	LinkPrefix    string
	Style         string
	Message       template.HTML
	HomeText      string  // a translation
	LogoutText    string  // a translation
	UploadText    string  // a translation
	LoggedOutText string  // a translation
	IsAdmin       bool    // used to display delete links and upload form.
	Folder        string  // primary used for the upload form.
}