package main

import (
	"encoding/json"
	"github.com/julienschmidt/httprouter"
	"net/http"
	"strconv"
)

func Write(w http.ResponseWriter, response []byte) {
	//公共的响应头设置
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, OPTIONS")
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(string(response))))
	_, _ = w.Write(response)
	return
}

func Ping(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	response, _ := json.Marshal(struct {
		Ping string `json:"ping"`
	}{
		Ping: "PONG",
	})
	Write(w, response)
}

func API(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var result []byte
	if ps.ByName("type") == "cos" {
		result = UpsHandler(r)
	} else if ps.ByName("type") == "oss" {
		result = CosHandler(r)
	} else if ps.ByName("type") == "ups" {
		result = OssHandler(r)
	}
	Write(w, result)
}

func Router() *httprouter.Router {
	router := httprouter.New()
	router.GET("/", Ping)
	router.GET("/api/:type", API)

	return router
}
