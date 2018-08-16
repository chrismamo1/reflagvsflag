package assets

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
)

func UploadImage(file multipart.File, header *multipart.FileHeader) string {
	// https://medium.com/@questhenkart/s3-image-uploads-via-aws-sdk-with-golang-63422857c548
	awsAccessKey := os.ExpandEnv("${AWS_ACCESS_KEY}")
	awsSecret := os.ExpandEnv("${AWS_SECRET}")
	token := ""
	creds := credentials.NewStaticCredentials(awsAccessKey, awsSecret, token)
	_, err := creds.Get()
	if err != nil {
		log.Println("Error setting up AWS credentials: ", err)
	}

	cfg := aws.NewConfig().WithRegion("us-east-1").WithCredentials(creds)
	svc := s3.New(session.New(), cfg)

	/// read the multipart file into a byte array and record its size
	// set the buffer capacity to 4MB
	bufferCap := 4 * (1 << 20)
	buffer := make([]byte, bufferCap)
	var nRead int64 = 0
	var n int = 1
	for n > 0 {
		upper := nRead + 4096
		n, err := file.Read(buffer[nRead:upper])
		nRead = nRead + int64(n)
		if err != nil {
			log.Println("Error reading the multipart file into a byte array: ", err)
			break
		}
	}
	log.Println("Image has size of ", nRead, " bytes")
	fileBytes := bytes.NewReader(buffer[0:nRead])
	fileType := http.DetectContentType(buffer[0:nRead])
	if strings.HasSuffix(header.Filename, ".svg") {
		fileType = "image/svg+xml"
	}

	/// take a hash of the image data and use it for the image path
	// just using the image data has fucky behaviour when people try to upload
	// identical images (some historical countries have had flags that are/were
	// the same solid color).
	h := md5.New()
	h.Write(buffer[0:nRead])
	// we fix this with a salt
	rand.Seed(time.Now().UTC().UnixNano())
	saltLen := 8 // 8 bytes should be enough
	salt := make([]byte, saltLen)
	for i, _ := range salt {
		salt[i] = byte(rand.Uint32() % 256)
	}
	h.Write(salt)
	checkSum := fmt.Sprintf("%x", h.Sum(nil))

	path := "/user-flags/" + checkSum + "_" + header.Filename
	params := &s3.PutObjectInput{
		Bucket:        aws.String("all-flags-of-sovereign-states-may2017"),
		Key:           aws.String(path),
		Body:          fileBytes,
		ContentLength: aws.Int64(nRead),
		ContentType:   aws.String(fileType),
		ACL:           aws.String("public-read"),
	}
	_, err = svc.PutObject(params)
	if err != nil {
		log.Println("Error uploading an object to S3: ", err)
	}
	return checkSum + "_" + header.Filename
}
