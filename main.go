package presigns3

import (
	"encoding/json"
	"log"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/julienschmidt/httprouter"
	"github.com/kthomas/go-aws-config"
	"github.com/satori/go.uuid"
)

var config = awsconf.GetConfig()

func main() {
	router := httprouter.New()
	router.GET("/", PresignS3Handler)

	log.Fatal(http.ListenAndServe("0.0.0.0:8080", router))
}

func render(obj interface{}, status int, w http.ResponseWriter) {
	w.Header().Set("content-type", "application/json; charset=UTF-8")
	w.WriteHeader(status)
	if &obj != nil && status != http.StatusNoContent {
		if err := json.NewEncoder(w).Encode(obj); err != nil {
			panic(err)
		}
	} else {
		w.Header().Set("content-length", "0")
	}
}

func renderError(message string, status int, w http.ResponseWriter) {
	err := map[string]*string{}
	err["message"] = &message
	render(err, status, w)
}

type PresignedS3Request struct {
	Metadata	*map[string]*string	`json:"metadata"`
	SignedHeaders	*map[string]*string	`json:"signed_headers"`
	Url		string			`json:"url"`
}

func PresignS3Handler(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	PresignS3(w, r)
}

func PresignS3(w http.ResponseWriter, r *http.Request) {
	awsConfig := &aws.Config{
		Region: aws.String(*config.DefaultRegion),
	}

	svc := s3.New(session.New(awsConfig))
	result, err := svc.ListBuckets(&s3.ListBucketsInput{})

	if err == nil {
		for _, bucket := range result.Buckets {
			bucketName := aws.StringValue(bucket.Name)
			if bucketName == aws.StringValue(config.DefaultS3Bucket) {
				fileparts := strings.Split(r.URL.Query().Get("filename"), ".")
				extension := "." + fileparts[len(fileparts) - 1]
				mimeType := mime.TypeByExtension(extension)
				key := uuid.NewV4().String() + extension

				metadata := map[string]*string{}
				decoder := json.NewDecoder(strings.NewReader(r.URL.Query().Get("metadata")))
				err := decoder.Decode(&metadata)

				r, _ := svc.PutObjectRequest(&s3.PutObjectInput{
					Bucket: aws.String(bucketName),
					ContentType: aws.String(mimeType),
					Key: aws.String(key),
					Metadata: metadata,
				})
				presignedUrl, err := r.Presign(5 * time.Minute)

				if err == nil {
					signedHeaders := map[string]*string{}
					for key := range r.SignedHeaderVals {
						val := r.HTTPRequest.Header.Get(key)
						signedHeaders[key] = &val
					}

					presigned := &PresignedS3Request{
						Metadata: &metadata,
						SignedHeaders: &signedHeaders,
						Url: presignedUrl,
					}
					render(presigned, http.StatusOK, w)
				} else {
					renderError("Internal Server Error", http.StatusInternalServerError, w)
				}

				break
			}
		}
	} else {
		renderError("Internal Server Error", http.StatusInternalServerError, w)
	}
}
