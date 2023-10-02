package main

import (
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	//"golang.org/x/net/proxy"
)

const position_file = ".fPosition.log"
const db_dir = ".domains.db/"
const ru_file = ".TITLES.txt"
const log_file = ".log"

const cThreads = 1000
const cChunk = 50
const pFile = 100000
const tOut = time.Second * 6

var fPosition = 0
var nThreads = 0

var stop bool = false

var cAllSites = 0
var cOkSites = 0
var cRuSites = 0

var speed = 0

var zone string = ""

var re = regexp.MustCompile("(?i)<title[^>]*>([^<]+)</title>")
var reRu = regexp.MustCompile("(?i)[а-я]{5,}")

// var reRu = regexp.MustCompile("(?i)(binary.*option|forex|invest|trading|trader|бинар.*опцион|форекс|инвест|трейдинг|трейдер)")
var reNoPrint = regexp.MustCompile(`[\x00-\x1F\x7F-\x9F]`)
var reSpace = regexp.MustCompile(" {2,}")
var reZone = regexp.MustCompile(`^[^\.]+\.`)

func main() {

	var domains []string
	var position = 0

	data, err := os.ReadFile(position_file)
	if err == nil {
		fPosition, _ = strconv.Atoi(string(data))
	}

	domains = dLoad()

	go FPS()

	for !stop {

		for nThreads < cThreads {

			var chunkDomains []string

			for i := 0; i < cChunk; i++ {
				chunkDomains = append(chunkDomains, domains[position])
				position++

				if position >= pFile {
					position = 0
					fPosition++
					domains = dLoad()
					//
					os.WriteFile(position_file, []byte(strconv.Itoa(fPosition)), 0644)
					//
				}
			}

			nThreads++
			go bot(chunkDomains)
		}

		time.Sleep(time.Second)
	}
	//
	os.WriteFile(".stop.log", []byte(""), 0644)
	//

}

func bot(chunkDomains []string) {

	var ok []string
	var domain string

	for i := 0; i < cChunk; i++ {

		cAllSites++
		speed++

		domain = strings.TrimSpace(chunkDomains[i])
		url := "https://" + domain + "/"
		//ref := "https://8pw.ru/"

		//
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}
		//req.Header.Set("Referer", ref)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/117.0")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		req.Header.Set("Accept-Language", "ru-RU,ru;q=0.8,en-US;q=0.5,en;q=0.3")

		//dialer, _ := proxy.SOCKS5("tcp", "127.0.0.1:1080", nil, proxy.Direct)

		httpClient := &http.Client{
			Timeout: tOut,
			Transport: &http.Transport{
				//Dial:            dialer.Dial,
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		response, err := httpClient.Do(req)
		if err != nil {
			continue
		}
		//

		if response.StatusCode != 200 {
			response.Body.Close()
			continue
		}

		res, err := io.ReadAll(response.Body)
		if err != nil {
			response.Body.Close()
			continue
		}

		page := string(res)
		response.Body.Close()

		//

		match := re.FindStringSubmatch(page)
		if match == nil {
			continue
		}

		cOkSites++

		title := match[1]

		match = reRu.FindStringSubmatch(title)
		if match == nil {
			continue
		}

		title = html.UnescapeString(title)
		title = reNoPrint.ReplaceAllString(title, " ")
		title = reSpace.ReplaceAllString(title, " ")
		title = strings.TrimSpace(title)

		match = reRu.FindStringSubmatch(title)
		if match == nil {
			continue
		}

		ok = append(ok, title+" \t "+domain)
	}

	zone = reZone.ReplaceAllString(domain, "")
	cRuSites += len(ok)

	if len(ok) > 0 {
		file, _ := os.OpenFile(ru_file, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		for _, line := range ok {
			fmt.Fprintln(file, line)
		}
		file.Close()
	}
	nThreads--
}

func FPS() {

	for {

		time.Sleep(time.Second * 60)

		speedString := strconv.Itoa(speed/60) + " / " + strconv.Itoa(speed*1440) + " / " + strconv.Itoa(speed*43200)
		speed = 0

		proc := float64(fPosition) / 2612.0 * 100.0
		procOk := float64(cOkSites) / float64(cAllSites) * 100.0
		procRu := float64(cRuSites) / float64(cOkSites) * 100.0

		resultString := fmt.Sprintf("%d   %d = %.0f %%   %d / %d = %.0f %%   Ru: %d = %.0f %%   %s   %s",
			nThreads, fPosition, proc, cAllSites, cOkSites, procOk, cRuSites, procRu, speedString, zone)

		//fmt.Println(resultString)

		//
		file, _ := os.OpenFile(log_file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		file.WriteString(resultString + "\n")
		file.Close()
		//

	}
}

func dLoad() []string {

	file, err := os.Open(db_dir + strconv.Itoa(fPosition) + ".gz")
	if err != nil {
		stop = true
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		stop = true
	}
	defer gzReader.Close()

	data, err := io.ReadAll(gzReader)
	if err != nil {
		stop = true
	}

	lines := strings.Split(string(data), "\n")

	return lines
}
