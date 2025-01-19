// This code is a Go version of a Python script that downloads images from a specified source.

package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const SALT = "laxiaoheiwu"
const COUNT = 10

func main() {
	id := flag.Int("id", 39492857, "PhotoPlus ID (e.g., 87654321)")
	count := flag.Int("count", COUNT, "Number of photos to download")
	flag.Parse()

	if *id > 0 {
		getAllImages(*id, *count)
	} else {
		fmt.Println("Wrong ID")
	}
}

func objKeySort(obj map[string]interface{}) string {
	var sortedKeys = make([]string, 0, len(obj))
	for key := range obj {
		sortedKeys = append(sortedKeys, key)
	}

	sort.Strings(sortedKeys)
	var newObj = make([]string, 0, len(sortedKeys))
	for _, key := range sortedKeys {
		if obj[key] != nil {
			value := fmt.Sprintf("%v", obj[key])
			newObj = append(newObj, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return strings.Join(newObj, "&")
}

var re = regexp.MustCompile(`[<>:"/\\|?*]`)

func sanitizeFilename(filename string) string {
	return re.ReplaceAllString(filename, "_")
}

func getAllImages(id int, count int) {
	t := time.Now().UnixNano() / int64(time.Millisecond) // Current timestamp in milliseconds
	dir := fmt.Sprintf("./dist/%d", id)

	data := map[string]interface{}{
		"activityNo": id,
		"isNew":      false,
		"count":      count,
		"page":       1,
		"ppSign":     "live",
		"picUpIndex": "",
		"_t":         t,
	}

	dataSort := objKeySort(data)
	h := md5.New()
	io.WriteString(h, dataSort+SALT)
	sign := hex.EncodeToString(h.Sum(nil))

	req, err := http.NewRequest("GET", "https://live.photoplus.cn/pic/pics", nil)
	if err != nil {
		log.Print(err)
		return
	}

	q := req.URL.Query()
	q.Add("_s", sign)
	for k, v := range data {
		q.Add(k, fmt.Sprintf("%v", v))
	}
	req.URL.RawQuery = q.Encode()

	fmt.Println(req.URL.String())

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Print(err)
		return
	}
	defer response.Body.Close()

	var rsp DetailsResponse
	if err := json.NewDecoder(response.Body).Decode(&rsp); err != nil {
		log.Print(err)
		return
	}

	fmt.Printf("Total photos: %d, target download: %d\n", rsp.Result.PicsTotal, count)

	if rsp.Code == 0 || !rsp.Success || rsp.Result.PicsTotal == 0 {
		fmt.Println("failed to get pic list")
		return
	}

	os.MkdirAll(dir, os.ModePerm)

	sort.Slice(rsp.Result.PicsArray, func(i, j int) bool {
		return rsp.Result.PicsArray[i].Id > rsp.Result.PicsArray[j].Id
	})
	downloadAllImages(rsp.Result.PicsArray, dir)
}

func downloadAllImages(picsArrays []PicsArray, dir string) {
	var wg sync.WaitGroup

	for _, pic := range picsArrays {
		rawUrl := fmt.Sprintf("https:%s", pic.OriginImg)
		wg.Add(1)
		go downloadImage(rawUrl, dir, &wg)
	}

	wg.Wait()
}

func downloadImage(reqUrl string, dir string, wg *sync.WaitGroup) {
	// fmt.Println(reqUrl)
	defer wg.Done()

	var filename string
	if parsedUrl, _ := url.Parse(reqUrl); parsedUrl != nil {
		filename = filepath.Base(parsedUrl.Path)
	} else {
		filename = filepath.Base(reqUrl)
	}

	filename = sanitizeFilename(filename)
	imagePath := filepath.Join(dir, filename)

	if _, err := os.Stat(imagePath); !os.IsNotExist(err) {
		return
	}

	response, err := http.Get(reqUrl)
	if err != nil {
		return
	}
	defer response.Body.Close()

	if response.StatusCode == 200 {
		file, err := os.Create(imagePath)
		if err != nil {
			return
		}
		defer file.Close()

		_, err = io.Copy(file, response.Body)
	}
}

type DetailsResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Result  Result `json:"result"`
}

type PicsArray struct {
	OriginImg string `json:"origin_img"`
	Id        int
}
type Result struct {
	PageTotal float64     `json:"pageTotal"`
	PicsTotal int         `json:"pics_total"`
	ViewCount int         `json:"view_count"`
	PicsArray []PicsArray `json:"pics_array"`
}
