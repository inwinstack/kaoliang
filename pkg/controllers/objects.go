package controllers

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/inwinstack/kaoliang/pkg/utils"
	"github.com/minio/minio/cmd"
)

type CopyObject struct {
	Bucket     string `json:"bucket"`
	CopySource string `json:"copy_source"`
	Key        string `json:"key"`
}

type MoveResult struct {
	Success bool
	Src     string
	Dest    string
	Code    string
	Message string
}

type MoveObject struct {
	Src  string
	Dest string
}

type MoveError struct {
	Src     string `json:"src"`
	Dest    string `json:"dest"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type MoveObjectsResponse struct {
	Moved []MoveObject `json:"moved"`
	Error []MoveError  `json:"error"`
}

func deleteObject(client *s3.S3, bucket, key string) error {
	deleteObjectInput := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	_, err := client.DeleteObject(deleteObjectInput)
	if err != nil {
		return err
	}

	return nil
}

func writeErrorResult(result *MoveResult, err error) {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case s3.ErrCodeObjectNotInActiveTierError:
			result.Code = s3.ErrCodeObjectNotInActiveTierError
			result.Message = aerr.Error()
		default:
			result.Code = aerr.Code()
			result.Message = aerr.Error()
		}
	} else {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		result.Message = err.Error()
	}
}

func MoveObjects(c *gin.Context) {
	_, errCode := authenticate(c.Request)
	if errCode != cmd.ErrNone {
		writeErrorResponse(c, errCode)
		return
	}

	accessKey := ExtractAccessKey(c.Request)
	_, creds, errCode := cmd.GetCredentials(accessKey)
	if errCode != cmd.ErrNone {
		writeErrorResponse(c, errCode)
		return
	}

	var objects []CopyObject
	if err := c.ShouldBindJSON(&objects); err != nil {
		writeErrorResponse(c, cmd.ErrMalformedJSON)
		return
	}

	sess, _ := session.NewSession(&aws.Config{
		Region:     aws.String(utils.GetEnv("RGW_REGION", "us-east-1")),
		Endpoint:   aws.String(utils.GetEnv("TARGET_HOST", "http://127.0.0.1:7480")),
		DisableSSL: aws.Bool(true),
		Credentials: credentials.NewStaticCredentials(
			creds.AccessKey,
			creds.SecretKey,
			"",
		),
		S3ForcePathStyle: aws.Bool(true),
	})

	client := s3.New(sess)
	numOfObjs := len(objects)

	jobs := make(chan CopyObject, numOfObjs)
	results := make(chan MoveResult, numOfObjs)

	for i := 1; i <= runtime.NumCPU(); i++ {
		go func(jobs <-chan CopyObject, results chan<- MoveResult) {
			for job := range jobs {
				result := MoveResult{
					Success: false,
					Src:     job.CopySource,
					Dest:    fmt.Sprintf("%s/%s", job.Bucket, job.Key),
				}

				input := &s3.CopyObjectInput{
					Bucket:     aws.String(job.Bucket),
					CopySource: aws.String(job.CopySource),
					Key:        aws.String(job.Key),
				}

				_, err := client.CopyObject(input)
				if err != nil {
					writeErrorResult(&result, err)
					results <- result
					continue
				}

				src := job.CopySource
				srcBucket := src[:strings.Index(src, "/")]
				srcKey := src[strings.Index(src, "/")+1:]
				// Delete source after successfully copy it to destination
				if err = deleteObject(client, srcBucket, srcKey); err != nil {
					if err = deleteObject(client, job.Bucket, job.Key); err != nil {
						writeErrorResult(&result, err)
						results <- result
						continue
					}

					results <- result
					continue
				}

				result.Success = true
				results <- result
			}
		}(jobs, results)
	}

	for _, obj := range objects {
		jobs <- obj
	}

	close(jobs)

	respBody := MoveObjectsResponse{
		Moved: []MoveObject{},
		Error: []MoveError{},
	}

	for i := 1; i <= numOfObjs; i++ {
		result := <-results
		if result.Success {
			respBody.Moved = append(respBody.Moved, MoveObject{
				Src:  result.Src,
				Dest: result.Dest,
			})
		} else {
			respBody.Error = append(respBody.Error, MoveError{
				Src:     result.Src,
				Dest:    result.Dest,
				Code:    result.Code,
				Message: result.Message,
			})
		}
	}

	c.JSON(200, respBody)
}
