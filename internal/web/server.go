// Lab 7: Implement a web server

package web

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
	// "web/templat"
)

type server struct {
	Addr string
	Port int

	metadataService VideoMetadataService
	contentService  VideoContentService

	mux *http.ServeMux
}
type VideoData struct {
	Id         string
	EscapedId  string
	UploadTime string
}

func NewServer(
	metadataService VideoMetadataService,
	contentService VideoContentService,
) *server {
	return &server{
		metadataService: metadataService,
		contentService:  contentService,
	}
}

func (s *server) Start(lis net.Listener) error {
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/upload", s.handleUpload)
	s.mux.HandleFunc("/videos/", s.handleVideo)
	s.mux.HandleFunc("/content/", s.handleVideoContent)
	s.mux.HandleFunc("/", s.handleIndex)

	return http.Serve(lis, s.mux)
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	videos, err := s.metadataService.List()
	if err != nil {
		log.Printf("error listing: %v", err)
	}
	var data []VideoData
	for _, video := range videos {
		data = append(data, VideoData{
			Id:         video.Id,
			EscapedId:  url.PathEscape(video.Id),
			UploadTime: video.UploadedAt.Format("Jan 2 2006 15:04"),
		})
		// renderTemplate(w, indexHTML, data)
	}
	tmpl, err := template.New("index").Parse(indexHTML)
	if err != nil {
		http.Error(w, "Template parsing error", http.StatusInternalServerError)
		return
	}

	// err = tmpl.Execute(w, data)
	// if err != nil {
	// 	http.Error(w, "Template execution error", http.StatusInternalServerError)
	// 	return
	// }
	err = tmpl.Execute(w, data)
	if err != nil {
		log.Println("Template execution error:", err)
		// Do not call http.Error here since response might be partially written
	}

	// panic("Lab 7: not implemented")
}

func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Get video ID (filename without extension)
	videoId := strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))

	// Check for duplicate
	existing, err := s.metadataService.Read(videoId)
	if err != nil {
		http.Error(w, "Failed to read metadata", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		http.Error(w, "Video ID already exists", http.StatusConflict)
		return
	}

	// Save uploaded file temporarily
	tempDir, err := os.MkdirTemp("", "upload-*")
	if err != nil {
		http.Error(w, "Failed to create temp dir", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, header.Filename)
	out, err := os.Create(tempFile)
	if err != nil {
		http.Error(w, "Failed to create temp file", http.StatusInternalServerError)
		return
	}
	defer out.Close()
	io.Copy(out, file)

	// Run ffmpeg to generate MPEG-DASH files
	manifestPath := filepath.Join(tempDir, "manifest.mpd")
	cmd := exec.Command("ffmpeg",
		"-i", tempFile,
		"-c:v", "libx264",
		"-c:a", "aac",
		"-bf", "1",
		"-keyint_min", "120",
		"-g", "120",
		"-sc_threshold", "0",
		"-b:v", "3000k",
		"-b:a", "128k",
		"-f", "dash",
		"-use_timeline", "1",
		"-use_template", "1",
		"-init_seg_name", "init-$RepresentationID$.m4s",
		"-media_seg_name", "chunk-$RepresentationID$-$Number%05d$.m4s",
		"-seg_duration", "4",
		manifestPath)
	if err := cmd.Run(); err != nil {
		http.Error(w, "FFmpeg failed", http.StatusInternalServerError)
		return
	}
	//delete vid after chunking
	if err := os.Remove(tempFile); err != nil {
		log.Printf("Failed to remove original video file: %v", err)
		// Not fatal, so don't return
	}
	log.Println("FFMPEG DONE ---------------")

	// Save files to content store
	// files, err := os.ReadDir(tempDir)

	// for _, file := range files {
	// 	// err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
	// 	// 	if info.IsDir() {
	// 	// 		log.Println("LOG LINE 1 -------------")
	// 	// 		return nil
	// 	// 	}
	// 		data, err := os.ReadFile(path)
	// 		if err != nil {
	// 			log.Println("LOG LINE 2 -------------")
	// 			return err
	// 		}
	// 		log.Println("LOG LINE 3 -------------")
	// 		filename := filepath.Base(path)
	// 		return s.contentService.Write(videoId, filename, data)
	// 	})
	// 	if err != nil {
	// 		log.Println("LOG LINE 4 -------------")

	// 		http.Error(w, "Failed to store files", http.StatusInternalServerError)
	// 		log.Println("error while uploading err = ", err)
	// 		return
	// 	}
	// }
	//do os.remove
	files, err := os.ReadDir(tempDir)
	if err != nil {
		log.Println("LOG LINE 4 -------------")
		http.Error(w, "Failed to list files", http.StatusInternalServerError)
		log.Println("error while uploading err = ", err)
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue // skip directories
		}
		path := filepath.Join(tempDir, file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Println("LOG LINE 2 -------------")
			http.Error(w, "Failed to read file", http.StatusInternalServerError)
			return
		}
		log.Println("LOG LINE 3 -------------")
		filename := file.Name()
		if err := s.contentService.Write(videoId, filename, data); err != nil {
			log.Println("LOG LINE 4 -------------")
			http.Error(w, "Failed to store files: "+err.Error(), http.StatusInternalServerError)
			log.Println("Failed to store file:", filename, "error:", err)
			log.Println("error while uploading err = ", err)
			return
		}
	}

	// Save metadata
	err = s.metadataService.Create(videoId, time.Now())
	if err != nil {
		http.Error(w, "Failed to store metadata: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
	// panic("Lab 7: not implemented")
}

func (s *server) handleVideo(w http.ResponseWriter, r *http.Request) {
	videoId := r.URL.Path[len("/videos/"):]
	log.Println("Video ID:", videoId)
	meta, err := s.metadataService.Read(videoId)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if meta == nil {
		http.Error(w, "404 not found", http.StatusNotFound)
		return
	}
	// err = renderTemplate(w, videoHTML, struct {
	// 	ID string
	// }{
	// 	ID: videoId,
	// })
	// if err != nil {
	// 	http.Error(w, "Failed to render page", http.StatusInternalServerError)
	// }

	tmpl, err := template.New("video").Parse(videoHTML)
	if err != nil {
		http.Error(w, "Template parsing error", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, struct {
		Id         string
		UploadedAt string
	}{
		Id:         videoId,
		UploadedAt: meta.UploadedAt.Format("2006-01-02 15:04:05"),
	})
	if err != nil {
		http.Error(w, "Template execution error", http.StatusInternalServerError)
		return
	}

	// defer

	// panic("Lab 7: not implemented")
}

func (s *server) handleVideoContent(w http.ResponseWriter, r *http.Request) {
	// parse /content/<videoId>/<filename>
	videoId := r.URL.Path[len("/content/"):]
	parts := strings.Split(videoId, "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid content path", http.StatusBadRequest)
		return
	}
	videoId = parts[0]
	filename := parts[1]
	log.Println("Video ID:", videoId, "Filename:", filename)
	// panic("Lab 7: not implemented")
	data, err := s.contentService.Read(videoId, filename)
	if err != nil {
		http.Error(w, "Failed to read video content", http.StatusInternalServerError)
		return
	}

	// Write the file content to the response
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
