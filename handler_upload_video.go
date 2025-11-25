package main

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
		token, err := auth.GetBearerToken(r.Header)	
		if err != nil {
			respondWithError(w,401,err.Error(),err)
			return
		}
		id, err := auth.ValidateJWT(token,cfg.jwtSecret)
		if err != nil {
			respondWithError(w,403,err.Error(),err)
			return
		}
		videoIDString := r.PathValue("videoID")
		videoId, err := uuid.Parse(videoIDString)
		if err != nil {
			respondWithError(w,400,err.Error(),err)
			return
		}
		const max = 10 << 30
		
		r.Body = http.MaxBytesReader(w,r.Body,max)

		video, err := cfg.db.GetVideo(videoId)
		if err != nil {
			respondWithError(w,400,err.Error(),err)
			return
		}
		if video.UserID != id{
			respondWithError(w,403,err.Error(),err)
			return
		}
		fd, fh, err := r.FormFile("video")
		if err != nil {
			respondWithError(w,400,err.Error(),err)
			return
		}
		defer fd.Close()

		contentType := fh.Header.Get("Content-Type")
		
		mime, _, err := mime.ParseMediaType(contentType)

		if err != nil {
			respondWithError(w,400,err.Error(),err)
			return
		}

		if mime != "video/mp4"{
			respondWithError(w,400,err.Error(),err)
			return
		}
		tf, err := os.CreateTemp("","tubely-upload.mp4")
		if err != nil {
			respondWithError(w,500,err.Error(),err)
			return
		}
		defer os.Remove(tf.Name())
		defer tf.Close()
		_, err = io.Copy(tf,fd) 
		if err != nil {
			respondWithError(w,500,err.Error(),err)
			return
		}
		_, err = tf.Seek(0, io.SeekStart)	
		if err != nil  {
			respondWithError(w,500,err.Error(),err)
			return
		}
		rand := "0df347646cb03975fd573b2716151f62.mp4"
		_, err = cfg.s3Client.PutObject(context.Background(),&s3.PutObjectInput{
			Bucket: &cfg.s3Bucket,
			Key: &rand,
			Body: tf,
			ContentType: &mime,

		})
		if err != nil {
			respondWithError(w,500,err.Error(),err)
			return
		}
		vurl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",cfg.s3Bucket,cfg.s3Region,rand)
		video.VideoURL = &vurl
		err = cfg.db.UpdateVideo(video)
		if err != nil {
			respondWithError(w,500,err.Error(),err)
			return
		}
		respondWithJSON(w,200,struct{}{})

	}
