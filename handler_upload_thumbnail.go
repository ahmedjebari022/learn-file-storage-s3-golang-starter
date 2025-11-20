package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}


	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w,400,err.Error(),err)
		return
	}
	fileData, fileHeader, err :=  r.FormFile("thumbnail")
	if err != nil{
		respondWithError(w,400,err.Error(),err)
		return
	}
	
	contentType := fileHeader.Header.Get("Content-Type")
	
	// data, err := io.ReadAll(fileData)
	// if err != nil {
	// 	respondWithError(w,400,err.Error(),err)
	// 	return
	// }
	mime, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		respondWithError(w,400,err.Error(),err)
		return
	}
	if mime != "image/jpeg" && mime != "image/png"{
		respondWithError(w,400,"only jpeg or png format ",fmt.Errorf(""))
		return
	}


	split := strings.Split(contentType,"/")
	fmt.Println(split[1])
	b := make([]byte,36)
	rand.Read(b)
	
	thumnailPath := filepath.Join(cfg.assetsRoot,fmt.Sprintf("%s.%s",base64.RawURLEncoding.EncodeToString(b),split[1]))
	fmt.Println(thumnailPath)
	image, err := os.Create(thumnailPath)
	
	if err != nil {
		respondWithError(w,500,err.Error(),err)
		return	
	}

	if _, err = io.Copy(image, fileData); err != nil {
		respondWithError(w,500,err.Error(),err)
		return
	}
	
	metadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w,400,err.Error(),err)
		return
	}
	if metadata.UserID != userID{
		respondWithJSON(w,http.StatusUnauthorized,struct{}{})
		return
	}
	thumbnailUrl := fmt.Sprintf("http://localhost:%s/assets/%s.%s",cfg.port,base64.RawURLEncoding.EncodeToString(b),split[1])
	fmt.Println(thumbnailUrl)
	err = cfg.db.UpdateVideo(database.Video{
		ThumbnailURL:&thumbnailUrl,
		ID: metadata.ID,
		UpdatedAt: time.Now(),
		CreatedAt: metadata.CreatedAt,
		VideoURL: metadata.VideoURL,
		CreateVideoParams: metadata.CreateVideoParams,
	})

	if err != nil {
		respondWithError(w,400,err.Error(),err)
		return
	}

	updatedVideo, err := cfg.db.GetVideo(videoID)
	fmt.Printf("video:%v",updatedVideo)
	if err != nil {
		respondWithError(w,500,err.Error(),err)
		return
	}
	respondWithJSON(w, http.StatusOK, updatedVideo)
}
