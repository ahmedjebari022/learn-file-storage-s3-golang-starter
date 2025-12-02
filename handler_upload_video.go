package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
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
			respondWithError(w,500,err.Error(), err)
			return
		}
		processed, err := processVideoForFastStart(tf.Name())
		if err != nil {
			respondWithError(w, 500, err.Error(), err)
			return 
		}
		ptf, err := os.Open(processed)
		if err != nil {
			respondWithError(w, 500, err.Error(), err)
			return
		}
		defer os.Remove(ptf.Name())
		defer ptf.Close()
		ratio, err := getVideoAspectRatio(tf.Name())
		if err != nil {
			respondWithError(w, 500, err.Error(), err)
			return 
		}
		
		rand := "0df347646cb03975fd593b2716150f72.mp4"
		prefix := ""
		switch ratio{
		case "16:9" :
			prefix = "landscape"
		case "9:16" :
			prefix = "portrait"
		default :
			prefix = "other"
		}
		fmt.Print(ratio)
		key := prefix + "/" + rand
		_, err = cfg.s3Client.PutObject(context.Background(),&s3.PutObjectInput{
			Bucket: &cfg.s3Bucket,
			Key: &key,
			Body: ptf,
			ContentType: &mime,

		})
		if err != nil {
			respondWithError(w,500,err.Error(),err)
			return
		}
		vurl := "https://" + cfg.s3CfDistribution+ "/" + key
		video.VideoURL = &vurl

		err = cfg.db.UpdateVideo(video)
		if err != nil {
			respondWithError(w,500,err.Error(),err)
			return
		}
		respondWithJSON(w,200,struct{}{})

	}


func getVideoAspectRatio(filepath string) (string, error){
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	b := bytes.Buffer{}
	cmd.Stdout = &b
	cmd.Run()
	type whJson struct{
		Streams []struct {
			Width 	int `json:"width"`
			Height 	int `json:"height"`
		}
	}
	var res whJson
	err := json.Unmarshal(b.Bytes(), &res)
	if err != nil {
		return "", err
	}
	fmt.Printf("width: %d, height :%d", res.Streams[0].Width, res.Streams[0].Height)
	if res.Streams[0].Width / 16 == res.Streams[0].Height / 9{
		return "16:9", nil 
	}
	if res.Streams[0].Width / 9 == res.Streams[0].Height / 16{
		return "9:16", nil
	}
	return "other", nil

}



func processVideoForFastStart (filePath string) (string, error){
	outputPath := filePath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputPath)
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return outputPath, nil
}
