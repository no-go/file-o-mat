package main

import (
	"html/template"
)

type Index struct {
	Title         string
	BaseURL       string
	LinkPrefix    string
	Style         string
	Message       template.HTML
	HomeText      string
	LogoutText    string
	UploadText    string
	LoggedOutText string
	IsAdmin       bool
	Folder        string
}