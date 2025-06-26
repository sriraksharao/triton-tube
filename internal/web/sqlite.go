// Lab 7: Implement a SQLite video metadata service

package web

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteVideoMetadataService struct {
	db *sql.DB
}

func NewSQLiteVideoMetadataService(dbPath string) (*SQLiteVideoMetadataService, error) {
	db, err := sql.Open("sqlite3", dbPath)
	// db:=sql.OpenDB()
	if err != nil {
		return nil, err
	}
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS videos (
		id TEXT PRIMARY KEY,
		uploaded_at TIMESTAMP
	);`
	if _, err := db.Exec(createTableQuery); err != nil {
		return nil, err
	}
	// return db, err
	return &SQLiteVideoMetadataService{db: db}, nil
}

// Uncomment the following line to ensure SQLiteVideoMetadataService implements VideoMetadataService
var _ VideoMetadataService = (*SQLiteVideoMetadataService)(nil) // To make sure that *SQLiteVideoMetadataService implements all methods required by VideoMetadataService

func (s *SQLiteVideoMetadataService) Create(videoId string, uploadedAt time.Time) error {
	_, err := s.db.Exec("insert into videos (id, uploaded_at) values(?,?)", videoId, uploadedAt)
	if err != nil {
		log.Printf("error inserting into db: %v", err)
		return err
	}

	return nil
}
func (s *SQLiteVideoMetadataService) Read(videoId string) (*VideoMetadata, error) {
	// uploadedAt := time.Time
	// _, err := s.db.Exec("insert into videos (id, uploaded_at) values(?,?)", videoId, uploadedAt)
	row := s.db.QueryRow("select id, uploaded_at from videos where id=?", videoId)
	// if err != nil {
	// 	log.Println("error fetching from db: %v", err)
	// 	return nil, nil
	// }
	var id string
	var uploadedAt time.Time
	err := row.Scan(&id, &uploadedAt)
	// if err != nil {
	// 	return nil, nil
	// }
	if err == sql.ErrNoRows {
		return nil, nil // not found
	}
	if err != nil {
		log.Printf("error reading from db: %v", err)
		return nil, err
	}
	return &VideoMetadata{Id: id, UploadedAt: uploadedAt}, nil

	// return nil
}
func (s *SQLiteVideoMetadataService) List() ([]VideoMetadata, error) {
	row, err := s.db.Query("Select id, uploaded_at from videos order by uploaded_at desc")
	if err != nil {
		log.Printf("error listing videos: %v", err)
		return nil, err
	}
	var videos []VideoMetadata
	for row.Next() {
		var id string
		var uploadedAt time.Time
		if err := row.Scan(&id, &uploadedAt); err != nil {
			return nil, err
		}
		videos = append(videos, VideoMetadata{Id: id, UploadedAt: uploadedAt})
	}

	return videos, nil
}
