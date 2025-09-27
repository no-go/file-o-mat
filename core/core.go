// Package provides functions to run a minimalistic file server as website.
// Special requests:
//  - ?xxxx: loads `etc/xxxx.css`, if xxxx is set as style in `etc/config.json`
//  - ?delete: delete a file given by the GET before "?"
//  - ?logout: logs out = give a StatusUnauthorized to the client
//
// See [Readme.md](https://github.com/no-go/file-o-mat) for examples, hints and ideas.
package core

import (
	"fmt"
	"net/http"
	"golang.org/x/crypto/bcrypt"
	"io"
	"io/ioutil"
	"os"
	"log/slog"
	"sync"
	"time"
	"path/filepath"
	"strings"
	"regexp"
	"encoding/json"
	"html/template"
)

// Define a map to store a single locales translation. See json files in `locales/`.
type Translations map[string]string

// These mini templates are used, to format each file line in `reqHandler()`.
const (
	DELETE_TMPL = "<a href=\"%s%s%s?delete\">[%s]</a>    "
	FOLDER_TMPL = "<a href=\"%s%s%s/\">%s &gt;</a>\n"
	FILE_TMPL   = "<a href=\"%s%s%s\">%s</a> (%0.2f kB)\n"
)

// Ip counter for login fails.
var failedLogins = make(map[string]int)
// Holds banned ip and how log they are banned.
var banList      = make(map[string]time.Time)
// Semaphore to sync requests on maps.
var mu           sync.Mutex
// The map to hold a translation.
var translations Translations
// Important configs handled by `config.go` and `etc/config.json`.
var Cfg          *Config

// function loadTranslations to load a initial translation from locales folder.
func LoadTranslations(lang string) error {
	file, err := os.Open(filepath.Join("locales", lang + ".json"))
	if err != nil {
		return err
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(bytes, &translations); err != nil {
		return err
	}
	return nil
}

// true, if password and login found in hash table Cfg.Users
func checkPassword(username, password string) bool {
	hashedPassword, exists := Cfg.Users[username]
	if !exists {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// just check, if the username is configured (`etc/config.json`) as admin.
// It does NOT check the login or password!
func isAdmin(username string) bool {
	return username == Cfg.AdminUser
}

// simplify the filename given by the client. Special: a file without extension gets a '.nix'
func sanitizeFilename(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) < 2 {
		re := regexp.MustCompile(`[^a-z0-9_-]`)
		return re.ReplaceAllString(strings.ToLower(filename), "_") + ".nix"
	}

	ext := parts[len(parts)-1]
	name := strings.Join(parts[:len(parts)-1], "_")

	re := regexp.MustCompile(`[^a-z0-9_-]`)
	safeName := re.ReplaceAllString(strings.ToLower(name), "_")
	safeExt := re.ReplaceAllString(strings.ToLower(ext), "")

	return fmt.Sprintf("%s.%s", safeName, safeExt)
}

// handles a POST and stores the file in valid path, if its byte is lower than `UploadMax` (config.json)
func handleFilePost(w http.ResponseWriter, r *http.Request, username string) bool {
	r.Body = http.MaxBytesReader(w, r.Body, Cfg.UploadMax)
	lastSlashIndex := strings.LastIndex(r.URL.Path, "/")
	dir := r.URL.Path[len(Cfg.BaseURL)+len(Cfg.LinkPrefix):lastSlashIndex+1]

	err := r.ParseMultipartForm(Cfg.UploadMax)
	if err != nil {
		slog.Error(err.Error())
		errorMessage := fmt.Sprintf(translations["limitHint"], Cfg.UploadMax)
		renderPage(w, r, http.StatusRequestEntityTooLarge, errorMessage, username)
		return false
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		slog.Error(err.Error())
		renderPage(w, r, http.StatusBadRequest, translations["postErr"], username)
		return false
	}
	defer file.Close()

	filename := sanitizeFilename(fileHeader.Filename)
	cleanPath := filepath.Clean(filepath.Join(Cfg.DataFolder, dir, filename))

	if !isPathAllowed(cleanPath) {
		hint := fmt.Sprintf(translations["pathErr"], cleanPath)
		slog.Warn(hint)
		renderPage(w, r, http.StatusNotFound, hint, username)
		return false
	}

	out, err := os.Create(cleanPath)
	if err != nil {
		slog.Error(err.Error())
		renderPage(w, r, http.StatusInternalServerError, translations["newErr"], username)
		return false
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		slog.Error(err.Error())
		renderPage(w, r, http.StatusInternalServerError, translations["saveErr"], username)
		return false
	}
	return true
}

// It handles all requests with early brakepoints, if the request...
//  - ignored / ban
//  - not logged in
//  - is admin
//  - posts a file
//  - path not allowed
//  - loggs out
//  - is a request to a style file
//  - file or path not found
//  - delete request
//  - a valid path with files in it
func ReqHandler(w http.ResponseWriter, r *http.Request) {
	ip := r.RemoteAddr

	mu.Lock()
	defer mu.Unlock()

	if blockedUntil, blocked := banList[ip]; blocked && time.Now().Before(blockedUntil) {
		// ignore request, because it is in ban list
		return
	}

	// check login
	username, password, ok := r.BasicAuth()
	if !ok || !checkPassword(username, password) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		slog.Warn(username + " is unauthorized")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		// count fails
		failedLogins[ip]++

		if failedLogins[ip] > Cfg.MaxFailed {
			// move fails cousing ip to ban list
			banList[ip] = time.Now().Add(Cfg.BlockDuration())
			delete(failedLogins, ip)
			slog.Warn("ban IP " + ip)
		}
		return
	}

	// extract path from request
	filePath := r.URL.Path[len(Cfg.BaseURL)+len(Cfg.LinkPrefix):]
	cleanPath := filepath.Clean(filepath.Join(Cfg.DataFolder, filePath))
	lastSlashIndex := strings.LastIndex(filePath, "/")
	dir := filePath[:lastSlashIndex+1]

	// post a file?
	if r.Method == http.MethodPost && isAdmin(username) {
		if !handleFilePost(w, r, username) {
			return
		}
		cleanPath = filepath.Join(Cfg.DataFolder, dir)
	}

	if !isPathAllowed(cleanPath) {
		hint := fmt.Sprintf(translations["pathErr"], cleanPath)
		slog.Warn(hint)
		renderPage(w, r, http.StatusNotFound, hint, username)
		return
	}

	// active logout request?
	if r.URL.RawQuery == "logout" {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		renderPage(w, r, http.StatusUnauthorized, translations["loggedOut"], "-")
		return
	}

	// style file request
	if r.URL.RawQuery == Cfg.Style {
		content, err := ioutil.ReadFile(filepath.Join("etc", Cfg.Style + ".css"))
		if err != nil {
			slog.Error(err.Error())
			renderPage(w, r, http.StatusNotFound, err.Error(), username)
			return
		}
		w.Header().Set("Content-Type", "text/css")
		w.Write(content)
		return
	}

	// file or dir not found
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		slog.Error(err.Error())
		renderPage(w, r, http.StatusNotFound, err.Error(), username)
		return
	}

	// handle a delete
	if (r.URL.RawQuery == "delete") && isAdmin(username) {
		os.Remove(cleanPath)
		slog.Info("delete file " + cleanPath)
		cleanPath = filepath.Join(Cfg.DataFolder, dir)
	}

	// get file infos
	fileInfo, _ := os.Stat(cleanPath)

	// is file/path a dir? list files!
	if fileInfo.IsDir() {
		files, err := os.ReadDir(cleanPath)
		if err != nil {
			slog.Error(err.Error())
			renderPage(w, r, http.StatusInternalServerError, translations["readErr"], username)
			return
		}
		var output strings.Builder

		for _, file := range files {
			fileName := file.Name()
			if !file.IsDir() {
				fileInfo, _ := file.Info()
				fileSize := float64(fileInfo.Size()) / 1024

				if isAdmin(username) {
					fmt.Fprintf(
					&output,
					DELETE_TMPL,
					Cfg.BaseURL,
					Cfg.LinkPrefix,
					dir + fileName,
					translations["delLink"])
				}

				fmt.Fprintf(
					&output,
					FILE_TMPL,
					Cfg.BaseURL,
					Cfg.LinkPrefix,
					dir + fileName,
					fileName,
					fileSize)
			} else {
				fmt.Fprintf(
					&output,
					FOLDER_TMPL,
					Cfg.BaseURL,
					Cfg.LinkPrefix,
					dir + fileName,
					fileName)
			}
		}
		renderPage(w, r, http.StatusOK, output.String(), username)
		return
	}

	// is path a file? make it downloadable
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(cleanPath))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, cleanPath)
}

// only filePath are allowed, which are inside the actual application folder
func isPathAllowed(path string) bool {
	absBaseDir, err := filepath.Abs(Cfg.DataFolder)
	if err != nil {
		return false
	}

	absFilePath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	return filepath.HasPrefix(absFilePath, absBaseDir)
}

// Function renders a page, if the request is not to strange or errorful.
// The string parameter `message` will not be escaped.
// It uses the `etc/xxxx` files as template (`template: xxxx` in `etc/config.json`).
func renderPage(w http.ResponseWriter, r *http.Request, status int, message string, username string) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "text/html")

	tmpl, err := template.ParseFiles(filepath.Join("etc", Cfg.Template))
	if err != nil {
		slog.Error(err.Error())
		fmt.Fprint(w, translations["tmplErr"])
		return
	}

	lastSlashIndex := strings.LastIndex(r.URL.Path, "/")
	data := Index{
		Title:            username,
		BaseURL:          Cfg.BaseURL,
		LinkPrefix:       Cfg.LinkPrefix,
		Style:            Cfg.Style,
		Message:          template.HTML(message),
		HomeText:         translations["homeLink"],
		LogoutText:       translations["logoutLink"],
		IsAdmin:          isAdmin(username),
		UploadText:       translations["uploadBtn"],
		LoggedOutText:    translations["loggedOut"],
		Folder:           r.URL.Path[len(Cfg.BaseURL)+len(Cfg.LinkPrefix):lastSlashIndex+1],
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		slog.Error(err.Error())
		fmt.Fprint(w, translations["tmplErr"])
	}
}

// a cleanup Task to fork through the ban list every `check_duration` minutes. See `etc/config.json`.
func Cleanup() {
	for {
		time.Sleep(Cfg.CheckDuration())
		mu.Lock()
		for ip, blockedUntil := range banList {
			if time.Now().After(blockedUntil) {
				delete(banList, ip)
				slog.Info("cleanup: free IP " + ip)
			}
		}
		mu.Unlock()
	}
}
