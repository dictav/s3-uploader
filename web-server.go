package main

import (
  "bytes"
  "crypto/hmac"
  "crypto/md5"
  "crypto/sha1"
  "encoding/base64"
  "fmt"
  "io/ioutil"
  "net/http"
  "os"
  "time"
)

const uploadMethod = "PUT"
const uploadContentType = "image/jpeg"
const maxFileSize = 1048576 // 1MB
const baseURL = "http://s3-ap-northeast-1.amazonaws.com/"

func uploadHandler(w http.ResponseWriter, r *http.Request) {
  fmt.Printf("Upload request from %s, ", r.RemoteAddr)
  ret := upload(w, r)
  fmt.Fprintf(w, ret)
  fmt.Println(ret)
}

func upload(w http.ResponseWriter, r *http.Request) (ret string) {

  // check method
  if r.Method != "POST" {
    w.WriteHeader(400)
    return "ERROR"
  }

  b, err := ioutil.ReadAll(r.Body)
  if err != nil {
    w.WriteHeader(500)
    return err.Error()
  }

  // check file size
  imageSize := len(b)
  if imageSize < 4 || imageSize > maxFileSize {
    w.WriteHeader(400)
    return fmt.Sprintf("IMAGE SIZE IS OUT OF RANGE ( 4 < size < %d )", maxFileSize)
  }
  fmt.Printf("upload file size (%d), ", len(b))

  // check jpeg file
  // http://www.wdic.org/w/TECH/JFIF
  // SOI : 0xFF 0xD8
  // EOI : 0xFF 0xD9
  if b[0] != 0xFF || b[1] != 0xD8 || b[imageSize-2] != 0xFF || b[imageSize-1] != 0xD9 {
    w.WriteHeader(400)
    return "SUPPORT ONLY JPEG FILE"
  }

  // set md5 to filename
  m := md5.New()
  _, err = m.Write(b)
  if err != nil {
    w.WriteHeader(500)
    return err.Error()
  }
  filename := fmt.Sprintf("%x.jpg", m.Sum(nil))

  //prepare AWSS3
  var (
    method      = "PUT"
    contentType = "image/jpeg"
    bucket      = os.Getenv("AWS_S3_BUCKET")
    path        = fmt.Sprintf("%s/%s", os.Getenv("AWS_S3_PREFIX_PATH"), filename)
    dateStr     = time.Now().Format(time.RFC1123)
  )
  stringToSign := method + "\n\n" + contentType + "\n" + dateStr + "\n/" + bucket + path
  mac := hmac.New(sha1.New, []byte(os.Getenv("AWS_SECRET")))
  mac.Write([]byte(stringToSign))
  data := mac.Sum(nil)
  auth := base64.StdEncoding.EncodeToString(data)

  body := bytes.NewBuffer(b)
  url := fmt.Sprintf("http://%s.s3.amazonaws.com%s", bucket, path)
  req, err := http.NewRequest(method, url, body)
  if err != nil {
    w.WriteHeader(500)
    return err.Error()
  }
  req.Header.Set("Date", dateStr)
  req.Header.Set("Content-type", contentType)
  req.Header.Set("Authorization", "AWS "+os.Getenv("AWS_KEY")+":"+auth)

  client := &http.Client{}
  res, err := client.Do(req)
  if err != nil {
    w.WriteHeader(res.StatusCode)
    return "ERROR"
  }

  return baseURL + bucket + path
}

const defaultPort = "8080"

func main() {
  http.HandleFunc("/upload", uploadHandler)
  if os.Getenv("AWS_KEY") == "" || os.Getenv("AWS_SECRET") == "" {
    println("REQUIRE AWS KEY AND SECRET !!!")
    return
  }
  if os.Getenv("AWS_S3_BUCKET") == "" {
    println("REQUIRE AWS BUCKET !!!")
    return
  }

  port := os.Getenv("PORT")
  if port == "" {
    port = defaultPort
  }
  println("running on port " + port)
  http.ListenAndServe(":"+port, nil)
}
