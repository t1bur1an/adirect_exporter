package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"net/http"
	"os"
	"strings"
	"time"
)

type AdirectAnswer struct {
	Type   string `json:"type"`
	Ticket string `json:"ticket"`
}

type AdirectClientAcc struct {
	GroupItems []struct {
		Name    string  `json:"name"`
		Value   float64 `json:"value"`
		Percent float64 `json:"percent"`
		Type    string  `json:"type"`
	} `json:"groupItems"`
	AccountCost float64 `json:"accountCost"`
	PapersCost  float64 `json:"papersCost"`
	Arrears     float64 `json:"arrears"`
	Code        int     `json:"code"`
	Message     string  `json:"message"`
}

var (
	login = os.Getenv("LOGIN")
	password = os.Getenv("PASSWORD")
	treaty = os.Getenv("TREATY")
	Distribution = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "stocks_distribution",
			Help: "distribution of stocks",
		},
		[]string{
			"type",
		},
	)
	accountMoney = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "account_money",
		Help: "Number of blob storage operations waiting to be processed.",
	},
		[]string{
			"name",
		},
	)
)

func regPromMetrics() {
	if ginPromErr := prometheus.Register(Distribution); ginPromErr != nil {
		panic(ginPromErr)
	}
	if ginPromErr := prometheus.Register(accountMoney); ginPromErr != nil {
		panic(ginPromErr)
	}
}

func adirect() *AdirectAnswer {
	loginResp := new(AdirectAnswer)

	url := "https://www.alfadirect.ru/api/account/authorize"
	method := "POST"

	payload := strings.NewReader("{\n	\"login\": \""+login+"\", \n	\"password\": \""+password+"\"\n}")

	client := &http.Client{
	}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		fmt.Println(err)
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&loginResp)
	if err != nil {
		panic(err)
	}

	return loginResp
}

func AdirectAmount(loginAnswer *AdirectAnswer) {
	accAmount := new(AdirectClientAcc)

	url := "https://lk.alfadirect.ru/api/client/chart/"+treaty
	method := "GET"

	client := &http.Client{
	}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		fmt.Println(err)
	}
	req.Header.Add("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Add("Accept-Encoding", "gzip, deflate, br")
	req.Header.Add("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7,bg;q=0.6,ca;q=0.5,fr;q=0.4,sq;q=0.3,de;q=0.2")
	req.Header.Add("Cache-Control", "no-cache")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Cookie", ".AD4AuthCookie="+loginAnswer.Ticket)
	req.Header.Add("Host", "lk.alfadirect.ru")
	req.Header.Add("Referer", "https://lk.alfadirect.ru/")
	req.Header.Add("Sec-Fetch-Dest", "empty")
	req.Header.Add("Sec-Fetch-Mode", "cors")
	req.Header.Add("Sec-Fetch-Site", "same-origin")

	res, err := client.Do(req)
	defer res.Body.Close()

	reader, err := gzip.NewReader(res.Body)
	defer reader.Close()
	if err != nil {
		panic(err)
	}

	err = json.NewDecoder(reader).Decode(&accAmount)
	if err != nil {
		panic(err)
	}
	accountMoney.WithLabelValues(login).Set(accAmount.AccountCost/100)
	for _, groupItems := range accAmount.GroupItems {
		Distribution.WithLabelValues(groupItems.Type).Set(groupItems.Value/100)
	}
}

func gatherData() {
	for {
		resp := adirect()
		AdirectAmount(resp)
		time.Sleep(10 * time.Minute)
	}
}

func main() {
	gin.DisableConsoleColor()
	r := gin.Default()
	p := ginprometheus.NewPrometheus("gin")
	p.Use(r)
	regPromMetrics()
	go gatherData()

	listen := os.Getenv("ADDRESS") + ":" + os.Getenv("PORT")
	err := r.Run(listen)
	if err != nil {
		panic(err)
	}
}
