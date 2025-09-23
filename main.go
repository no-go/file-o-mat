package main

import (
	"fmt"
	"net/http"
	"golang.org/x/crypto/bcrypt"
	"io"
	"io/ioutil"
	"os"
	"log"
	"log/slog"
	"sync"
	"time"
	"path/filepath"
	"strings"
	"context"
	"os/signal"
	"syscall"
	"regexp"
	"encoding/json"
	"html/template"
)

type Translations map[string]string

const (
	DELETE_TMPL = "<a href=\"%s?delete\">[%s]</a>    "
	FOLDER_TMPL = "<a href=\"%s/\">%s &gt;</a>\n"
	FILE_TMPL   = "<a href=\"%s\">%s</a> (%0.2f kB)\n"
)

var (
	// use...
	//   go run . secure_password_here
	// ...to get a valid password hash here!
	users = map[string]string {
		"admin": "$2a$10$m07vqvz.1a/8BXMj.15sme4l4O0/0uX3bySMJcE0d2TlykshrkFku",
		"tux": "$2a$10$MuUfb3OxA.U.M7Ea/cNNkOWfEcUihiae/wQwquArHMHa5gpbFsjbq",
	}

	failedLogins = make(map[string]int)
	banList      = make(map[string]time.Time)
	mu           sync.Mutex

	translations Translations
	cfg          *Config
)

func loadTranslations(lang string) error {
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

func checkPassword(username, password string) bool {
	hashedPassword, exists := users[username]
	if !exists {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

func isAdmin(username string) bool {
	return username == cfg.AdminUser
}

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

func handleFilePost(w http.ResponseWriter, r *http.Request, username string) bool {
	r.Body = http.MaxBytesReader(w, r.Body, cfg.UploadMax)
	lastSlashIndex := strings.LastIndex(r.URL.Path, "/")
	dir := r.URL.Path[len(cfg.BaseURL):lastSlashIndex+1]

	err := r.ParseMultipartForm(cfg.UploadMax)
	if err != nil {
		slog.Error(err.Error())
		errorMessage := fmt.Sprintf(translations["limitHint"], cfg.UploadMax)
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
	cleanPath := filepath.Clean(filepath.Join(cfg.DataFolder, dir, filename))

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

func reqHandler(w http.ResponseWriter, r *http.Request) {
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

		if failedLogins[ip] > cfg.MaxFailed {
			// move fails cousing ip to ban list
			banList[ip] = time.Now().Add(cfg.BlockDuration())
			delete(failedLogins, ip)
			slog.Warn("ban IP " + ip)
		}
		return
	}

	// extract path from request
	filePath := filepath.Join(cfg.DataFolder, r.URL.Path[len(cfg.BaseURL):])
	cleanPath := filepath.Clean(filePath)
	lastSlashIndex := strings.LastIndex(r.URL.Path, "/")
	dir := r.URL.Path[:lastSlashIndex+1]

	// post a file?
	if r.Method == http.MethodPost && isAdmin(username) {
		if !handleFilePost(w, r, username) {
			return
		}
		cleanPath = filepath.Join(cfg.DataFolder, dir[len(cfg.BaseURL):])
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
	if r.URL.RawQuery == cfg.Style {
		content, err := ioutil.ReadFile(filepath.Join("etc", cfg.Style + ".css"))
		if err != nil {
			slog.Error(err.Error())
			renderPage(w, r, http.StatusNotFound, err.Error(), username)
			return
		}
		w.Header().Set("Content-Type", "text/css")
		fmt.Fprintf(w, string(content))
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
		cleanPath = filepath.Join(cfg.DataFolder, dir[len(cfg.BaseURL):])
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
					dir + fileName,
					translations["delLink"])
				}

				fmt.Fprintf(
					&output,
					FILE_TMPL,
					dir + fileName,
					fileName,
					fileSize)
			} else {
				fmt.Fprintf(
					&output,
					FOLDER_TMPL,
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
func isPathAllowed(filePath string) bool {
	absBaseDir, err := filepath.Abs(cfg.DataFolder)
	if err != nil {
		return false
	}

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}

	return filepath.HasPrefix(absFilePath, absBaseDir)
}

func renderPage(w http.ResponseWriter, r *http.Request, status int, message string, username string) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "text/html")

	tmpl, err := template.ParseFiles(filepath.Join("etc", cfg.Template))
	if err != nil {
		slog.Error(err.Error())
		fmt.Fprintf(w, translations["tmplErr"])
		return
	}

	lastSlashIndex := strings.LastIndex(r.URL.Path, "/")
	data := Index{
		Title:            username,
		BaseURL:          cfg.BaseURL,
		Style:            cfg.Style,
		Message:          template.HTML(message),
		HomeText:         translations["homeLink"],
		LogoutText:       translations["logoutLink"],
		IsAdmin:          isAdmin(username),
		UploadText:       translations["uploadBtn"],
		LoggedOutText:    translations["loggedOut"],
		Folder:           r.URL.Path[:lastSlashIndex+1],
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		slog.Error(err.Error())
		fmt.Fprintf(w, translations["tmplErr"])
	}
}

func cleanup() {
	for {
		time.Sleep(cfg.CheckDuration())
		mu.Lock()
		for ip, blockedUntil := range banList {
			if time.Now().After(blockedUntil) {
				delete(banList, ip)
				slog.Info("cleanup: free IP " + ip)
			}
		}
		mu.Unlock()
		slog.Info("cleanup: done.")
	}
}

func main() {
	var err error

	if (len(os.Args) > 1) {
		// hack to display hash for a given password
		password := os.Args[1]
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			slog.Error(err.Error())
		}
		fmt.Println(string(hashedPassword))
		return
	}
	// load config
	cfg, err = LoadConfig(filepath.Join("etc", "config.json"))
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	// setup logging
	file, err := os.OpenFile(filepath.Join("etc", cfg.LogFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		slog.Error(err.Error())
	}
	defer file.Close()
	log.SetOutput(file)
	log.SetFlags(log.Ldate | log.Lmicroseconds)

	// string translations
	err = loadTranslations(cfg.Lang)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	// extra cleanup thread
	go cleanup()

	// configure and start server
	http.HandleFunc(cfg.BaseURL, reqHandler)

	server := &http.Server{Addr: ":" + cfg.Port, Handler: http.DefaultServeMux}

	// setup a stop signal for mainthread
	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, syscall.SIGINT, syscall.SIGTERM)

	// start server inside an extra thread
	go func() {
		slog.Info("start http server. handle " + cfg.BaseURL + " on port " + cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error(err.Error())
		}
		slog.Info("server thread end")
	}()

	// mainthread waits for a signal
	<-stopSignal

	slog.Info("http server shutdown...")
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()
	if err = server.Shutdown(ctx); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	// fine :-)
	slog.Info("http server stop.")
}
